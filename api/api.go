package api

import (
	"encoding/json"
	"fmt"
	log "github.com/cihub/seelog"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/ant0ine/go-json-rest/rest"
	"github.com/chenyf/gibbon/comet"
	"github.com/chenyf/gibbon/devcenter"
	"github.com/chenyf/gibbon/zk"
)

type ApiResponse struct {
	ErrNo  int         `json:"errno"`
	ErrMsg string      `json:"errmsg"`
	Data   interface{} `json:"data,omitempty"`
	//ErrMsg string      `json:"errmsg,omitempty"`
	//Data interface{} `json:"data"`
}

const (
	ERR_NOERROR     = 10000
	ERR_CMD_TIMEOUT = 20000
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

func Error(w rest.ResponseWriter, errNo int, errMsg string, code int) {
	resp := ApiResponse{
		ErrNo:  errNo,
		ErrMsg: errMsg,
	}
	b, _ := json.Marshal(resp)
	http.Error(w.(http.ResponseWriter), string(b), code)
}

func checkAuthz(uid string, devid string) bool {
	log.Tracef("checkAuthz")

	devices, err := devcenter.GetDevices(uid, devcenter.DEV_ROUTER)
	if err != nil {
		log.Errorf("GetDevices failed: %s", err.Error())
		return false
	}

	for _, dev := range devices {
		if devid == dev.Id {
			return true
		}
	}
	return false
}

func getStatus(w rest.ResponseWriter, r *rest.Request) {
	resp := ApiResponse{
		ErrNo: ERR_NOERROR,
		Data:  fmt.Sprintf("Total registered devices: %d", comet.DevMap.Size()),
	}
	w.WriteJson(resp)
}

func postRouterCommand(w http.ResponseWriter, r *http.Request) {
	log.Tracef("postRouterCommand")
	log.Debugf("Request from RemoterAddr: %s", r.RemoteAddr)
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
		log.Warnf("auth failed. uid: %s, rid: %s", uid, rid)
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
		response.Error = fmt.Sprintf("recv response timeout [%s]", client.DevId)
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
		devices  []devcenter.Device
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

	devices, err = devcenter.GetDevices(uid, devcenter.DEV_ROUTER)
	if err != nil {
		log.Errorf("GetDevices failed: %s", err.Error())
		response.Descr = err.Error()
		goto resp
	}

	for _, dev := range devices {
		router = RouterInfo{
			Rid:   dev.Id,
			Rname: dev.Title,
		}
		response.List = append(response.List, router)
	}

	//router = RouterInfo{
	//	Rid:   "c80e774a1e73",
	//	Rname: "router1",
	//}
	//response.List = append(response.List, router)

	response.Status = 0
	response.Descr = "OK"

resp:
	b, _ := json.Marshal(response)
	log.Debugf("getRoutelist write: %s", string(b))
	w.Write(b)
}

func getGibbon(w rest.ResponseWriter, r *rest.Request) {
	resp := ApiResponse{
		ErrNo: ERR_NOERROR,
		Data:  zk.GetComet(),
	}
	w.WriteJson(resp)
}

func getDevices(w rest.ResponseWriter, r *rest.Request) {
	devices := []string{}
	devMap := comet.DevMap.Items()
	for devId, _ := range devMap {
		devices = append(devices, devId.(string))
	}
	resp := ApiResponse{
		ErrNo: ERR_NOERROR,
		Data:  devices,
	}
	w.WriteJson(resp)
}

func getDevice(w rest.ResponseWriter, r *rest.Request) {
	type DevInfo struct {
		Id        string `json:"id"`
		LastAlive string `json:"last_alive"`
		RegTime   string `json:"reg_time"`
	}

	devId := r.PathParam("devid")
	if !comet.DevMap.Check(devId) {
		rest.NotFound(w, r)
		return
	}
	client := comet.DevMap.Get(devId).(*comet.Client)
	devInfo := DevInfo{
		Id:        client.DevId,
		LastAlive: client.LastAlive.String(),
		RegTime:   client.RegistTime.String(),
	}
	resp := ApiResponse{
		ErrNo: ERR_NOERROR,
		Data:  devInfo,
	}
	w.WriteJson(resp)
}

func controlDevice(w rest.ResponseWriter, r *rest.Request) {
	type ControlParam struct {
		Token  string `json:"token"`
		Worker string `json:"worker"`
		Cmd    string `json:"cmd"`
	}

	devId := r.PathParam("devid")
	if !comet.DevMap.Check(devId) {
		rest.NotFound(w, r)
		return
	}
	client := comet.DevMap.Get(devId).(*comet.Client)
	param := ControlParam{}
	err := r.DecodeJsonPayload(&param)
	if err != nil {
		log.Warnf("Error decode param: %s", err.Error())
		//Error(w, ERR_BAD_REQUEST, "Bad request", 400)
		rest.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	cmdRequest := CommandRequest{
		Cmd: param.Cmd,
	}

	resp := ApiResponse{}

	bCmd, _ := json.Marshal(cmdRequest)
	reply := make(chan *comet.Message)
	client.SendMessage(comet.MSG_REQUEST, bCmd, reply)
	select {
	case msg := <-reply:
		resp.ErrNo = ERR_NOERROR
		resp.Data = msg.Data
	case <-time.After(10 * time.Second):
		resp.ErrNo = ERR_CMD_TIMEOUT
		resp.ErrMsg = fmt.Sprintf("recv response timeout [%s]", client.DevId)
	}
	w.WriteJson(resp)
}

func StartHttp(addr string) {
	log.Infof("Starting HTTP server on %s", addr)

	handler := rest.ResourceHandler{}
	err := handler.SetRoutes(
		&rest.Route{"GET", "/devices", getDevices},
		&rest.Route{"GET", "/devices/:devid", getDevice},
		&rest.Route{"POST", "/devices/:devid", controlDevice},
		&rest.Route{"GET", "/servers", getGibbon},
		&rest.Route{"GET", "/status", getStatus},
	)
	if err != nil {
		log.Criticalf("http SetRoutes: ", err)
		os.Exit(1)
	}

	// the adapter API for old system
	http.HandleFunc("/router/command", postRouterCommand)
	http.HandleFunc("/router/list", getRouterList)

	// new API
	http.Handle("/api/v1/", http.StripPrefix("/api/v1", &handler))

	err = http.ListenAndServe(addr, nil)
	if err != nil {
		log.Criticalf("http listen: ", err)
		os.Exit(1)
	}
}
