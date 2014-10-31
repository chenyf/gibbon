package api

import (
	"encoding/json"
	"fmt"
	log "github.com/cihub/seelog"
	"net/http"
	"os"
	"time"

	"github.com/ant0ine/go-json-rest/rest"
	"github.com/chenyf/gibbon/comet"
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

var (
	commandTimeout int
)

func getStatus(w rest.ResponseWriter, r *rest.Request) {
	resp := ApiResponse{
		ErrNo: ERR_NOERROR,
		Data:  fmt.Sprintf("Total registered devices: %d", comet.DevMap.Size()),
	}
	w.WriteJson(resp)
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

	cmdRequest := comet.CommandRequest{
		Cmd: param.Cmd,
	}

	resp := ApiResponse{}

	bCmd, _ := json.Marshal(cmdRequest)
	reply := make(chan *comet.Message)
	client.SendMessage(comet.MSG_ROUTER_COMMAND, bCmd, reply)
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

func StartHttp(addr string, cmdTimeout int) {
	log.Infof("Starting HTTP server on %s, command timeout: %ds", addr, cmdTimeout)
	commandTimeout = cmdTimeout

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
