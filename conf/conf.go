package conf

import (
	"encoding/json"
	"os"
)

type ConfigStruct struct {
	Comet            string `json:"comet"`
	AcceptTimeout    int    `json:"accept_timeout"`
	ReadTimeout      int    `json:"read_timeout"`
	HeartbeatTimeout int    `json:"heartbeat_timeout"`

	Web       string `json:"web"`
	DevCenter string `json:"devcenter"`

	ZooKeeper struct {
		Enable  bool          `json:"enable"`
		Addr    string        `json:"addr"`
		Timeout time.Duration `json:"timeout"`
		Path    string        `json:"path"`
	} `json:"zookeeper"`
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
