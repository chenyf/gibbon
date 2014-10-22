package devcenter

import (
	"encoding/json"
	"errors"
	"fmt"
	log "github.com/cihub/seelog"
	"io/ioutil"
	"net/http"

	"github.com/chenyf/gibbon/conf"
)

const (
	DEV_ROUTER = 3
)

type Device struct {
	Id    string `json:"id"`
	Type  int    `json:"type"`
	Title string `json:"title"`
}

type devicesResult struct {
	Errno  int    `json:"errno"`
	Errmsg string `json:"errmsg"`
	Data   struct {
		UserOpenId int      `json:"userOpenId"`
		DeviceList []Device `json:"device"`
	} `json:"data"`
}

func GetDevices(uid string, devType int) ([]Device, error) {
	log.Tracef("GetDevices")
	url := fmt.Sprintf("http://%s/api/v1/device/bind/?user_id=%s&type=%d", conf.Config.DevCenter, uid, devType)
	res, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	log.Debugf("Got response from device center: %s", body)

	var result devicesResult
	err = json.Unmarshal(body, &result)
	if err != nil {
		return nil, err
	}

	if result.Errno != 10000 {
		return nil, errors.New(result.Errmsg)
	}

	return result.Data.DeviceList, nil
}
