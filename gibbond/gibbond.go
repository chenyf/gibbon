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

	"github.com/chenyf/gibbon/api"
	"github.com/chenyf/gibbon/comet"
	"github.com/chenyf/gibbon/conf"
	"github.com/chenyf/gibbon/zk"
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
	cometServer := comet.NewServer()

	listener, err := cometServer.Init(conf.Config.Comet)
	if err != nil {
		log.Criticalf("Failed to start comet server: %s", err.Error())
		os.Exit(1)
	}

	cometServer.SetAcceptTimeout(time.Duration(conf.Config.AcceptTimeout) * time.Second)
	cometServer.SetReadTimeout(time.Duration(conf.Config.ReadTimeout) * time.Second)
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

	if conf.Config.ZooKeeper.Enable {
		if err := zk.ProduceZnode(conf.Config.ZooKeeper.Addr,
			conf.Config.ZooKeeper.Root,
			conf.Config.ZooKeeper.CometAddr,
			time.Duration(conf.Config.ZooKeeper.Timeout)*time.Second); err != nil {
			log.Criticalf("ProduceZnode failed: %s", err.Error())
			os.Exit(1)
		}
		if err := zk.Watch(conf.Config.ZooKeeper.Addr,
			conf.Config.ZooKeeper.Root,
			time.Duration(conf.Config.ZooKeeper.Timeout)*time.Second); err != nil {
			log.Criticalf("Watch failed: %s", err.Error())
			os.Exit(1)
		}
	}

	go func() {
		cometServer.Run(listener)
	}()

	waitGroup.Add(1)
	go api.StartHttp(conf.Config.Web)
	waitGroup.Wait()
}

