package zk

import (
	"conf"
	"github.com/samuel/go-zookeeper/zk"
)

func InitZk() error {
	conn, eventCh, err := zk.Connect(conf.Config.ZooKeeper.Addr, conf.Config.ZooKeeper.Timeout*time.Second)
	if err != nil {
		return err
	}

	_, err := conn.Create(conf.Config.ZooKeeper.Path, nil, 0, zk.WorldACL(zk.PermAll))
	if err != nil && err != zk.ErrNodeExists {
		return err
	}
}
