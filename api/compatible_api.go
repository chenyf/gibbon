package api

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	log "github.com/cihub/seelog"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/chenyf/gibbon/comet"
	"github.com/chenyf/gibbon/devcenter"
)

func sign(path string, query map[string]string) []byte {
	uid := query["uid"]
	rid := query["rid"]
	tid := query["tid"]
	src := query["src"]
	tm := query["tm"]
	pmtt := query["pmtt"]

	raw := []string{path, uid, rid, tid, src, tm, pmtt}
	args := []string{}
	x := []int{6, 5, 4, 3, 2, 1, 0}
	for _, item := range x {
		args = append(args, raw[item])
	}
	data := strings.Join(args, "")
	key := "xnRzFxoCDRVRU2mNQ7AoZ5MCxpAR7ntnmlgRGYav"
	mac := hmac.New(sha1.New, []byte(key))
	mac.Write([]byte(data))
	return mac.Sum(nil)
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

	type RouterInfo struct {
		Rid   string `json:"rid"`
		Rname string `json:"rname"`
	}

	type ResponseRouterList struct {
		Status int          `json:"status"`
		Descr  string       `json:"descr"`
		List   []RouterInfo `json:"list"`
	}

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

	response.Status = 0
	response.Descr = "OK"

resp:
	b, _ := json.Marshal(response)
	log.Debugf("getRoutelist write: %s", string(b))
	w.Write(b)
}
