package zk

import (
	"strings"
	"time"

	"github.com/chenyf/gibbon/conf"
	log "github.com/cihub/seelog"
	"github.com/samuel/go-zookeeper/zk"
)

func InitZk() error {
	conn, _, err := zk.Connect(strings.Split(conf.Config.ZooKeeper.Addr, ","),
		time.Duration(conf.Config.ZooKeeper.Timeout)*time.Second)
	if err != nil {
		return err
	}

	// create the root znode for this app
	_, err = conn.Create(conf.Config.ZooKeeper.Root, nil, 0, zk.WorldACL(zk.PermAll))
	if err != nil && err != zk.ErrNodeExists {
		return err
	}

	// create the znode with the address of this app
	path, err := conn.Create(conf.Config.ZooKeeper.Root+"/",
		[]byte(conf.Config.ZooKeeper.CometAddr),
		zk.FlagEphemeral|zk.FlagSequence, zk.WorldACL(zk.PermAll))
	if err != nil {
		return err
	}

	log.Infof("znode created at [%s] with data [%s]", path, conf.Config.ZooKeeper.CometAddr)

	return nil
}
