package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	//"io"
	"github.com/chenyf/gibbon/comet"
	log "github.com/cihub/seelog"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	defaultLogConfig string = `
<seelog minlevel="trace">
	<outputs formatid="common">
		<console />
		<rollingfile type="size" filename="/tmp/agent.log" maxsize="20971520" maxrolls="1"/>
	</outputs>
	<formats>
		<format id="common" format="%Date/%Time [%LEV] %Msg%n" />
	</formats>
</seelog>
`
)

type MsgHandler func(*Conn, *comet.Header, []byte) int

type Agent struct {
	done    chan bool
	funcMap map[uint8]MsgHandler
}

func NewAgent() *Agent {
	agent := &Agent{
		done:    make(chan bool),
		funcMap: make(map[uint8]MsgHandler),
	}
	agent.funcMap[comet.MSG_REGISTER_REPLY] = handleRegisterReply
	agent.funcMap[comet.MSG_ROUTER_COMMAND] = handleRouterCommand
	return agent
}

func (this *Agent) Run() {
	var c *Conn = NewConn()
	addSlice := strings.Split(Config.Address, ";")
	for {
		select {
		case <-this.done:
			log.Infof("agent quit")
			return
		default:
		}
		if c.conn == nil {
			if ok := c.Connect(addSlice[0]); !ok {
				time.Sleep(5 * time.Second)
				continue
			}
			log.Infof("connect ok")
			c.Start()
		}
		c.conn.SetReadDeadline(time.Now().Add(3 * time.Second))
		n := c.Read()
		if n < 0 {
			// failed
			c.Close()
			c = NewConn()
			continue
		} else if n > 0 {
			// need more data
			continue
		}
		// ok
		if handler, ok := this.funcMap[c.header.Type]; ok {
			ret := handler(c, &c.header, c.dataBuf)
			if ret != 0 {
				c.Close()
				c = NewConn()
				continue
			}
		} else {
			log.Warnf("unknown message type %d", c.header.Type)
		}
		c.BufReset()
	}
}

func (this *Agent) Stop() {
	close(this.done)
}

func sendReply(c *Conn, msgType uint8, seq uint32, v interface{}) {
	b, _ := json.Marshal(v)
	c.SendMessage(msgType, seq, b, nil)
}

func handleRegisterReply(c *Conn, header *comet.Header, body []byte) int {
	return 0
}

func handleRouterCommand(c *Conn, header *comet.Header, body []byte) int {
	var msg comet.RouterCommandMessage
	var reply comet.RouterCommandReplyMessage

	if err := json.Unmarshal(body, &msg); err != nil {
		reply.Status = 2000
		reply.Descr = "Request body is not JSON"
		sendReply(c, comet.MSG_ROUTER_COMMAND_REPLY, header.Seq, &reply)
		return 0
	}
	if msg.Cmd.Forward == "" {
		reply.Status = 2001
		reply.Descr = "'forward' is empty"
		sendReply(c, comet.MSG_ROUTER_COMMAND_REPLY, header.Seq, &reply)
		return 0
	}
	client := &http.Client{
		Transport: &http.Transport{
			Dial: func(netw, addr string) (net.Conn, error) {
				conn, err := net.DialTimeout(netw, addr, time.Second*5)
				if err != nil {
					return nil, err
				}
				conn.SetDeadline(time.Now().Add(time.Second * 2))
				return conn, nil
			},
			ResponseHeaderTimeout: time.Second * 2,
		},
	}

	response, err := client.Post("http://127.0.0.1:9999/",
		"application/json;charset=utf-8",
		bytes.NewBuffer([]byte(msg.Cmd.Forward)))
	if err != nil {
		reply.Status = 2002
		reply.Descr = "Talk with local service failed"
		sendReply(c, comet.MSG_ROUTER_COMMAND_REPLY, header.Seq, &reply)
		return 0
	}
	result, err := ioutil.ReadAll(response.Body)
	response.Body.Close()
	if err != nil {
		reply.Status = 2003
		reply.Descr = "Local service response failed"
		sendReply(c, comet.MSG_ROUTER_COMMAND_REPLY, header.Seq, &reply)
		return 0
	}
	reply.Status = 0
	reply.Descr = "OK"
	reply.Result = string(result)
	sendReply(c, comet.MSG_ROUTER_COMMAND_REPLY, header.Seq, &reply)
	return 0
}

type Server struct {
	DeviceId string
}
type Serverslice struct {
	Servers []Server
}

type Pack struct {
	msg   *comet.Message
	reply chan *comet.Message
}
type Conn struct {
	conn     *net.TCPConn
	done     chan bool
	outMsgs  chan *Pack
	readFlag int
	nRead    int
	headBuf  []byte
	dataBuf  []byte
	header   comet.Header
}

func NewConn() *Conn {
	return &Conn{
		conn:     nil,
		done:     make(chan bool),
		outMsgs:  make(chan *Pack, 100),
		readFlag: 0,
		nRead:    0,
		headBuf:  make([]byte, comet.HEADER_SIZE),
	}
}

func (this *Conn) Connect(service string) bool {
	log.Infof("try to connect server address: %v\n", service)
	tcpAddr, err := net.ResolveTCPAddr("tcp4", service)
	if err != nil {
		log.Infof("resolve tcp address fail:%v\n", err)
		return false
	}

	conn, err := net.DialTCP("tcp", nil, tcpAddr)
	if err != nil {
		log.Infof("connect failed, (%s)", err)
		return false
	}
	conn.SetNoDelay(true)
	this.conn = conn
	return true
}

func (this *Conn) Start() {
	macAddr, _ := GetMac()
	b := []byte(macAddr)
	this.SendMessage(comet.MSG_REGISTER, 0, b, nil)
	go func() {
		timer := time.NewTicker(110 * time.Second)
		h := comet.Header{}
		h.Type = comet.MSG_HEARTBEAT
		heartbeat, _ := h.Serialize()
		for {
			select {
			case <-this.done:
				return
			case pack := <-this.outMsgs:
				//seqid := pack.client.nextSeq
				//pack.msg.Header.Seq = seqid
				b, _ := pack.msg.Header.Serialize()
				this.conn.Write(b)
				this.conn.Write(pack.msg.Data)
				log.Infof("send msg: (%d) (%s)", pack.msg.Header.Type, pack.msg.Data)
				//pack.client.nextSeq += 1
				time.Sleep(1 * time.Second)
			case <-timer.C:
				this.conn.Write(heartbeat)
			}
		}
	}()
}

func (this *Conn) Read() int {
	var n int
	if this.readFlag == 0 {
		n = MyRead(this.conn, this.headBuf[this.nRead:])
		if n < 0 {
			return -1
		} else if n == 0 {
			return 1
		}
		this.nRead += n
		if uint32(this.nRead) < comet.HEADER_SIZE {
			return 1
		}

		if err := this.header.Deserialize(this.headBuf[0:comet.HEADER_SIZE]); err != nil {
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
	n = MyRead(this.conn, this.dataBuf[this.nRead:])
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

func (this *Conn) BufReset() {
	this.readFlag = 0
	this.nRead = 0
}

func (this *Conn) Close() {
	this.done <- true
	this.conn.Close()
	this.conn = nil
	this.BufReset()
}

func (this *Conn) SendMessage(msgType uint8, seq uint32, body []byte, reply chan *comet.Message) {
	header := comet.Header{
		Type: msgType,
		Ver:  0,
		Seq:  seq,
		Len:  uint32(len(body)),
	}
	msg := &comet.Message{
		Header: header,
		Data:   body,
	}
	pack := &Pack{
		msg:   msg,
		reply: reply,
	}
	this.outMsgs <- pack
}

func main() {
	var (
		logConfigFile = flag.String("l", "", "Log config file")
		configFile    = flag.String("c", "/system/etc/conf.json", "Config file")
	)

	flag.Parse()

	err := LoadConfig(*configFile)
	if err != nil {
		fmt.Printf("LoadConfig from %s failed: (%s)\n", *configFile, err)
		os.Exit(1)
	}

	var logger log.LoggerInterface
	if *logConfigFile == "" {
		logger, _ = log.LoggerFromConfigAsBytes([]byte(defaultLogConfig))
	} else {
		//logger, err := log.LoggerFromConfigAsFile("/system/etc/log.xml")
		logger, err = log.LoggerFromConfigAsFile(*logConfigFile)
		if err != nil {
			fmt.Printf("Load log config from %s failed: (%s)\n", *logConfigFile, err)
			os.Exit(1)
		}
	}
	log.ReplaceLogger(logger)

	//wg := &sync.WaitGroup{}
	agent := NewAgent()
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		agent.Run()
		log.Infof("agent quited")
		wg.Done()
	}()
	sig := <-c
	log.Infof("Received signal '%v', exiting\n", sig)
	agent.Stop()
	wg.Wait()
}
