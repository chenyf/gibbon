package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/widuu/gojson"
	"io"
	"io/ioutil"
	log "github.com/cihub/seelog"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
	"github.com/chenyf/gibbon/util"
	"github.com/chenyf/gibbon/conf"
)

type MsgHandler func(*net.TCPConn, *Header, []byte) int
type Pack struct {
	msg    *Message
	client *Client
	reply  chan *Message
}

type Agent struct {
	done	chan bool
	funcMap       map[uint8]MsgHandler
}

func NewAgent() *Agent {
	agent := &Agent{
		done: make(chan bool),
		funcMap: make(map[uint8]MsgHandler),
	}
	agent.funcMap[MSG_REGISTER_REPLY] = handlerRegisterReply
	agent.funcMap[MSG_COMMAND]        = handlerCommnd
	return agent
}

func (this *Agent)Run() {
	var c Conn
	addSlice = strings.Split(Config.Address)
	for {
		select {
			case this->done:
				break
			default:
		}
		if c.conn == nil {
			if ok := c.Make(addSlice[0]); !ok {
				time.Sleep(1*time.Seconds)
				continue
			}
			c.Start()
		}
		n := c.Read()
		if n < 0 {
		// failed
			c.Close()
			continue
		} else if n > 0 {
		// need more data
			continue
		}
		// ok
		if handler, ok := this.funcMap[c.header.Type]; ok {
			handler(c.conn, &c.header, c.dataBuf)
		} else {
			log.Warnf("unkonw")
		}
		c.BufReset()
	}
}

func (this *Agent)Stop() {
	this->done <- true
}

func handleRegisterReply() {
	return 0
}

func handleCommand() {
	return 0
}

type Server struct {
	DeviceId string
}
type Serverslice struct {
	Servers []Server
}

type CommandHttpResponse struct {
	Status uint8  `json:"status"`
	Result string `json:"result"`
	Descr  string `json:"descr"`
}

type Conn struct {
	conn *net.TCPConn
	readFlag int
	nRead    int
	headBuf	 []byte
	dataBuf  []byte
	header   Header
}

func NewConn() *Conn {
	return &Conn{
		headBuf = make([]byte, HEADER_SIZE)
	}
}

func (this *Conn)Make(service string) bool {
	log.Infof("try to connect server address :%v\n", service)
	tcpAddr, err := net.ResolveTCPAddr("tcp4", service)
	if err != nil {
		log.Infof("resolve tcp address fail:%v\n", err)
		return false
	}

	conn, err = net.DialTCP("tcp", nil, tcpAddr)
	if err != nil {
		return false
	}
	conn.SetNoDelay(true)
	return true
}

func (this *Conn)Start() {
	macAddr, err := util.GetMac()
	b := []byte(macAddr)
	this.SendMessage(MSG_REGISTER, b, nil)
	go func() {
        timer := time.NewTicker(60*time.Second)
		heartbeat := make([]byte, 1)
		heartbeat[0] = 0
		for {
			select {
			case <-this->done:
				return
			case msg := this->outMsgs:
                //seqid := pack.client.nextSeq
				//pack.msg.Header.Seq = seqid
				b, _ := pack.msg.Header.Serialize()
				conn.Write(b)
				conn.Write(pack.msg.Data)
				log.Infof("%s: send msg: (%d) (%s)", client.devId, pack.msg.Header.Type, pack.msg.Data)
				//pack.client.nextSeq += 1
				time.Sleep(0.1 * time.Second)
			case <- timer.C:
				conn.Write(heartbeat)
			}
		}
	}()
}

func (this *Conn)Read() int {
	if this.readFlag == 0 {
		n = myread(this.conn, this.headBuf[this.nRead:])
		if n < 0 {
			return -1
		} else if n == 0 {
			return 1
		}
		this.nRead += n
		if uint32(this.nRead) < HEADER_SIZE {
			return 1
		}

		if err := this.header.Deserialize(this.headBuf[0:HEADER_SIZE]); err != nil {
			return -1
		}

		if this.header.Len <= 0 {
			this.nRead = 0
			return 1
		}
		this.readFlag = 1
		this.dataBuf = make([]byte, this.header.Len)
		this.nRead = 0
	}
	n = myread(this.conn, this.dataBuf[this.nRead:])
	if n < 0 {
		return -1
	} else if n == 0 {
		return 1
	}
	this.nRead += n
	if uint32(this.nRead) < this.header.Len {
		return 1
	}
	return 0
}

func (this *Conn)BufReset() {
	this.readFlag = 0
	this.nRead = 0
}

func (this *Conn)Close() {
	this.done <- true
	this.conn.Close()
	this.conn = nil
	this.BufReset()
}

func (this *Conn)SendMessage(msgType uint8, body []byte, reply chan *Message) {
    header := Header{
		Type: msgType,
		Ver:  0,
		Seq:  0,
		Len:  uint32(len(body)),
	}
	msg := &Message{
		Header: header,
		Data:   body,
	}
	pack := &Pack{
		msg:    msg,
		client: client,
		reply:  reply,
	}
	this.outMsgs <- pack
}

func main() {
	err := LoadConfig("/system/etc/conf.json")
	if err != nil {
		fmt.Printf("LoadConfig failed: (%s)", err)
		os.Exit(1)
	}

	logger, err := log.LoggerFromConfigAsFile("/system/etc/log.xml")
	if err != nil {
		fmt.Printf("Load log config failed: (%s)\n", err)
		os.Exit(1)
	}
	log.ReplaceLogger(logger)

	wg := &sync.WaitGroup{}
	agent := NewAgent()
	c := make(chan os.Signal, 1)
    signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)

	go func() {
		agent.Run()
	}()
	sig := <-c
	agent.Stop()
}
