package zk

import (
	"strings"
	"time"

	log "github.com/cihub/seelog"
	"github.com/samuel/go-zookeeper/zk"
)

func ProduceZnode(zkAddr string, root string, cometAddr string, timeout time.Duration) error {
	conn, _, err := zk.Connect(strings.Split(zkAddr, ","), timeout)
	if err != nil {
		return err
	}
	log.Infof("connected to zk server [%s]", zkAddr)

	// create the root znode for this app
	path, err := conn.Create(root, nil, 0, zk.WorldACL(zk.PermAll))
	if err != nil {
		if err != zk.ErrNodeExists {
			return err
		}
		log.Infof("root znode [%s] already exists", root)
	} else {
		log.Infof("root znode created at [%s]", path)
	}

	// create the znode with the address of this app
	path, err = conn.Create(root+"/", []byte(cometAddr),
		zk.FlagEphemeral|zk.FlagSequence, zk.WorldACL(zk.PermAll))
	if err != nil {
		return err
	}
	log.Infof("znode created at [%s] with data [%s]", path, cometAddr)

	return nil
}
