package zk

import (
	"fmt"
	"strings"
	"time"

	log "github.com/cihub/seelog"
	"github.com/samuel/go-zookeeper/zk"
)

var (
	cometServerList = []string{}
)

func GetComet() []string {
	return cometServerList
}

func Watch(zkAddr string, root string, timeout time.Duration) error {
	conn, _, err := zk.Connect(strings.Split(zkAddr, ","), timeout)
	if err != nil {
		return err
	}
	log.Infof("connected to zk server [%s]", zkAddr)

	// create the root znode (if not exist) for this app
	path, err := conn.Create(root, nil, 0, zk.WorldACL(zk.PermAll))
	if err != nil {
		if err != zk.ErrNodeExists {
			return err
		}
		log.Infof("root znode [%s] already exists", root)
	} else {
		log.Infof("root znode created at [%s]", path)
	}

	_, _, eventCh, err := conn.ChildrenW(root)
	if err != nil {
		log.Errorf("watch children err: %s", err.Error())
		return err
	}
	log.Infof("added watch on [%s]", root)

	// init the cometServerList
	serverList, err := getServerList(conn, root)
	if err != nil {
		log.Errorf("get children err: [%s]", err.Error())
	} else {
		cometServerList = serverList
		log.Infof("Comet server list: %v", cometServerList)
	}

	go func() {
		// handle the znode event
		for {
			select {
			case event := <-eventCh:
				if event.Type == zk.EventNodeChildrenChanged {
					log.Infof("Children changed")
					serverList, err := getServerList(conn, root)
					if err != nil {
						continue
					}
					cometServerList = serverList
					log.Infof("Comet server list changed: %v", cometServerList)
				}
			}
		}
	}()

	return nil
}

func getServerList(conn *zk.Conn, root string) ([]string, error) {
	znodes, _, err := conn.Children(root)
	if err != nil {
		log.Errorf("get children err: [%s]", err.Error())
		return nil, err
	}
	serverList := []string{}
	for _, znode := range znodes {
		path := fmt.Sprintf("%s/%s", root, znode)
		data, _, err := conn.Get(path)
		if err != nil {
			log.Errorf("get data for znode [%s] err: [%s]", path, err.Error())
			continue
		}
		serverList = append(serverList, string(data))
	}
	return serverList, nil
}
