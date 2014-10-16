package api

import (
	"encoding/json"
	"fmt"
	log "github.com/cihub/seelog"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/chenyf/gibbon/comet"
	"github.com/chenyf/gibbon/devcenter"
)

type CommandRequest struct {
	Uid string `json:"uid"`
	Cmd string `json:"cmd"`
}

type CommandResponse struct {
	Status int    `json:"status"`
	Error  string `json:"error"`
}

type RouterInfo struct {
	Rid   string `json:"rid"`
	Rname string `json:"rname"`
}

type ResponseRouterList struct {
	Status int          `json:"status"`
	Descr  string       `json:"descr"`
	List   []RouterInfo `json:"list"`
}

func checkAuthz(uid string, devid string) bool {
	//TODO
	return true
}

func getStatus(w http.ResponseWriter, r *http.Request) {
	size := comet.DevMap.Size()
	fmt.Fprintf(w, "total register device: %d\n", size)
}

func postRouterCommand(w http.ResponseWriter, r *http.Request) {
	var (
		uid        string
		rid        string
		response   CommandResponse
		client     *comet.Client
		body       []byte
		err        error
		cmdRequest CommandRequest
		bCmd       []byte
		reply      chan *comet.Message
	)
	response.Status = 1
	if r.Method != "POST" {
		response.Error = "must using 'POST' method\n"
		goto resp
	}
	r.ParseForm()
	rid = r.FormValue("rid")
	if rid == "" {
		response.Error = "missing 'rid'"
		goto resp
	}

	uid = r.FormValue("uid")
	if uid == "" {
		response.Error = "missing 'uid'"
		goto resp
	}

	if !checkAuthz(uid, rid) {
		response.Error = "authorization failed"
		goto resp
	}

	/*
		uid := r.FormValue("uid")
		if uid == "" {	fmt.Fprintf(w, "missing 'uid'\n"); return; }
		tid := r.FormValue("tid")
		if tid == "" {	fmt.Fprintf(w, "missing 'tid'\n"); return; }
		sign := r.FormValue("sign")
		if sign == "" {	fmt.Fprintf(w, "missing 'sign'\n"); return; }
		tm := r.FormValue("tm")
		if tm == "" {	fmt.Fprintf(w, "missing 'tm'\n"); return; }
		pmtt := r.FormValue("pmtt")
		if pmtt == "" {	fmt.Fprintf(w, "missing 'pmtt'\n"); return; }
		query := map[string]string {
			"uid" : uid,
			"rid" : rid,
			"tid" : tid,
			"src" : "letv",
			"tm" :  tm,
			"pmtt" : pmtt,
		}
		path := "/router/command"
		mysign := sign(path, query)
		if mysign != sign {
			response.Error = "sign valication failed"
			b, _ := json.Marshal(response)
			fmt.Fprintf(w, string(b))
			return
		}
	*/

	if r.Body == nil {
		response.Error = "missing POST data"
		goto resp
	}

	if !comet.DevMap.Check(rid) {
		response.Error = fmt.Sprintf("device (%s) offline", rid)
		goto resp
	}
	client = comet.DevMap.Get(rid).(*comet.Client)

	body, err = ioutil.ReadAll(r.Body)
	r.Body.Close()
	if err != nil {
		response.Error = "invalid POST body"
		goto resp
	}

	cmdRequest = CommandRequest{
		Uid: uid,
		Cmd: string(body),
	}

	bCmd, _ = json.Marshal(cmdRequest)
	reply = make(chan *comet.Message)
	client.SendMessage(comet.MSG_REQUEST, bCmd, reply)
	select {
	case msg := <-reply:
		w.Write(msg.Data)
	case <-time.After(10 * time.Second):
		response.Error = "recv response timeout"
		goto resp
	}
	return

resp:
	b, _ := json.Marshal(response)
	log.Debugf("postRouterCommand write: %s", string(b))
	w.Write(b)
}

func getRouterList(w http.ResponseWriter, r *http.Request) {
	log.Tracef("getRouterList")
	var (
		uid      string
		tid      string
		response ResponseRouterList
		router   RouterInfo
		devices  map[string]devcenter.Device
		err      error
	)

	response.Status = -1
	if r.Method != "GET" {
		response.Descr = "must using 'GET' method\n"
		goto resp
	}
	r.ParseForm()

	uid = r.FormValue("uid")
	if uid == "" {
		response.Descr = "missing 'uid'"
		goto resp
	}

	tid = r.FormValue("tid")
	if tid == "" {
		response.Descr = "missing 'tid'"
		goto resp
	}

	devices, err = devcenter.GetDevices(uid)
	if err != nil {
		log.Errorf("GetDevices failed: %s", err.Error())
		response.Descr = err.Error()
		goto resp
	}

	for _, dev := range devices {
		log.Debugf("devid: %s, type: %d, title: %s", dev.Id, dev.Type, dev.Title)
		router = RouterInfo{
			Rid:   dev.Id,
			Rname: dev.Title,
		}
		response.List = append(response.List, router)
	}

	router = RouterInfo{
		Rid:   "c80e774a1e73",
		Rname: "router1",
	}

	response.Status = 0
	response.Descr = "OK"
	response.List = append(response.List, router)

resp:
	b, _ := json.Marshal(response)
	log.Debugf("getRoutelist write: %s", string(b))
	w.Write(b)
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
	client.SendMessage(comet.MSG_REQUEST, []byte(cmd), reply)
	select {
	case msg := <-reply:
		fmt.Fprintf(w, "recv reply  (%s)\n", string(msg.Data))
	case <-time.After(10 * time.Second):
		fmt.Fprintf(w, "recv timeout\n")
	}
}

func StartHttp(addr string) {
	log.Infof("Starting HTTP server on %s", addr)
	http.HandleFunc("/router/command", postRouterCommand)
	http.HandleFunc("/router/list", getRouterList)
	http.HandleFunc("/command", getCommand)
	http.HandleFunc("/status", getStatus)
	err := http.ListenAndServe(addr, nil)
	if err != nil {
		log.Criticalf("http listen: ", err)
		os.Exit(1)
	}
}
