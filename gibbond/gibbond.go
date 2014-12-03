package main

import (
	"flag"
	"fmt"
	//"log"
	log "github.com/cihub/seelog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/chenyf/gibbon/comet"
	"github.com/chenyf/gibbon/conf"
	"github.com/chenyf/gibbon/mq"
	"github.com/chenyf/gibbon/storage"
)

func main() {

	var (
		flConfig = flag.String("c", "./etc/conf.json", "Config file")
	)

	flag.Parse()

	err := conf.LoadConfig(*flConfig)
	if err != nil {
		fmt.Printf("LoadConfig (%s) failed: (%s)\n", *flConfig, err)
		os.Exit(1)
	}

	logger, err := log.LoggerFromConfigAsFile("./etc/log.xml")
	if err != nil {
		fmt.Printf("Load log config failed: (%s)\n", err)
		os.Exit(1)
	}

	log.ReplaceLogger(logger)

	waitGroup := &sync.WaitGroup{}

	storage.NewInstance(conf.Config.Redis.Server,
		conf.Config.Redis.Pass,
		conf.Config.Redis.PoolSize,
		conf.Config.Redis.Retry)

	cometServer := comet.NewServer()
	listener, err := cometServer.Init(conf.Config.Comet)
	if err != nil {
		log.Criticalf("Failed to start comet server: %s", err.Error())
		os.Exit(1)
	}

	cometServer.SetAcceptTimeout(time.Duration(conf.Config.AcceptTimeout) * time.Second)
	cometServer.SetReadTimeout(time.Duration(conf.Config.ReadTimeout) * time.Second)
	cometServer.SetHeartbeatInterval(time.Duration(conf.Config.HeartbeatInterval) * time.Second)
	cometServer.SetHeartbeatTimeout(time.Duration(conf.Config.HeartbeatTimeout) * time.Second)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)
	go func() {
		sig := <-c
		log.Infof("Received signal '%v', exiting\n", sig)
		///utils.RemovePidFile(srv.	runtime.config.Pidfile)
		cometServer.Stop()
		log.Infof("leave 1")
		waitGroup.Done()
		log.Infof("leave 2")
	}()

	go func() {
		cometServer.Run(listener)
	}()

	_, err = mq.NewRpcServer(conf.Config.Rabbit.Uri, "gibbon_rpc_exchange", "gibbon")
	if err != nil {
		log.Critical("failed to start RPC server: ", err)
		os.Exit(1)
	}

	waitGroup.Add(1)
	waitGroup.Wait()
}
