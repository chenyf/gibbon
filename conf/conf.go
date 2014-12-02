package conf

import (
	"encoding/json"
	"os"
)

type ConfigStruct struct {
	Comet             string `json:"comet"`
	AcceptTimeout     int    `json:"accept_timeout"`
	ReadTimeout       int    `json:"read_timeout"`
	HeartbeatInterval int    `json:"heartbeat_interval"`
	HeartbeatTimeout  int    `json:"heartbeat_timeout"`
	Rabbit            struct {
		Enable bool   `json:"enable"`
		Uri    string `json:"uri"`
	} `json:"rabbit"`
}

var (
	Config ConfigStruct
)

func LoadConfig(filename string) error {
	r, err := os.Open(filename)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(r)
	err = decoder.Decode(&Config)
	if err != nil {
		return err
	}
	return nil
}
