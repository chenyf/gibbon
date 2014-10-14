package conf

import (
	"encoding/json"
	"os"
)

type ConfigStruct struct {
	Comet string `json:"comet"`
	Web   string `json:"web"`
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
