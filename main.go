package main

import (
	"flag"
	"log"
	"sync"
	"os"
	"os/signal"
	"syscall"
	"net/http"
	"fmt"
	"time"
	//"github.com/chenyf/echoserver/engine"
	"github.com/chenyf/gibbon/comet"
)

func getStatus(w http.ResponseWriter, r *http.Request) {
	size := comet.DevMap.Size()
	fmt.Fprintf(w, "total register device: %d\n", size)
}

func getCommand(w http.ResponseWriter, r *http.Request) {

	r.ParseForm()
	devid := r.FormValue("devid")
	if devid == "" {
		fmt.Fprintf(w, "missing devid\n")
		return
	}
	if !comet.DevMap.Check(devid) {
		fmt.Fprintf(w, "(%s) not register\n", devid)
		return
	}
	cmd := r.FormValue("cmd")
	client := comet.DevMap.Get(devid).(*comet.Client)
	reply := make(chan *comet.Message)
	seqid := client.SendMessage(comet.MSG_REQUEST, []byte(cmd), reply)
	select {
	case msg := <-reply:
		delete(client.MsgFoo,  seqid)
		fmt.Fprintf(w, "recv reply  (%s)\n", string(msg.Data))
	case <- time.After(10 * time.Second):
		delete(client.MsgFoo,  seqid)
		fmt.Fprintf(w, "recv timeout\n")
	}
}

func main() {

	var (
		//flRoot               = flag.String("g", "/tmp/echoserver", "Path to use as the root of the docker runtime")
	)

	flag.Parse()

	/*job := eng.Job("init")
	if err := job.Run(); err != nil {
		log.Fatal(err)
	}
	*/

	waitGroup := &sync.WaitGroup{}
	cometServer := comet.NewServer()

	listener, err := cometServer.Init("0.0.0.0:10000")
	if err != nil {
		log.Fatal(err)
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)
	go func() {
		sig := <-c
		log.Printf("Received signal '%v', exiting\n", sig)
		///utils.RemovePidFile(srv.	runtime.config.Pidfile)
		cometServer.Stop()
		log.Printf("leave 1")
		waitGroup.Done()
		log.Printf("leave 2")
	}()

	go func() {
		cometServer.Run(listener)
	}()
	waitGroup.Add(1)
	go func() {
		http.HandleFunc("/command", getCommand)
		http.HandleFunc("/status", getStatus)
		err := http.ListenAndServe("0.0.0.0:9999", nil)
		if err != nil {
			log.Fatal("http listen: ", err)
		}
	}()
	/*
	job = eng.Job("restapi")
	job.SetenvBool("Logging", true)
	if err := job.Run(); err != nil {
		log.Fatal(err)
	}
	*/
	waitGroup.Wait()
}

