package comet

import (
	//"log"
	log "github.com/cihub/seelog"
	"io"
	"net"
	"sync"
	"time"
	//"strings"
	"github.com/chenyf/gibbon/utils/safemap"
)

type MsgHandler func(*Client, *Header, []byte) int

type Server struct {
	exitCh           chan bool
	waitGroup        *sync.WaitGroup
	funcMap          map[uint8]MsgHandler
	acceptTimeout    time.Duration
	readTimeout      time.Duration
	writeTimeout     time.Duration
	heartbeatTimeout time.Duration
	maxMsgLen        uint32
}

func NewServer() *Server {
	return &Server{
		exitCh:           make(chan bool),
		waitGroup:        &sync.WaitGroup{},
		funcMap:          make(map[uint8]MsgHandler),
		acceptTimeout:    2 * time.Second,
		readTimeout:      60 * time.Second,
		writeTimeout:     60 * time.Second,
		heartbeatTimeout: 90 * time.Second,
		maxMsgLen:        2048,
	}
}

type Client struct {
	devId           string
	ctrl            chan bool
	MsgOut          chan *Pack
	WaitingChannels map[uint32]chan *Message
	NextSeqId       uint32
	LastAlive       time.Time
}

type Pack struct {
	msg    *Message
	client *Client
	reply  chan *Message
}

func (client *Client) SendMessage(msgType uint8, body []byte, reply chan *Message) {
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
	client.MsgOut <- pack
}

var (
	DevMap *safemap.SafeMap = safemap.NewSafeMap()
)

func InitClient(conn *net.TCPConn, devid string) *Client {
	client := &Client{
		devId:           devid,
		ctrl:            make(chan bool),
		MsgOut:          make(chan *Pack, 100),
		WaitingChannels: make(map[uint32]chan *Message),
		NextSeqId:       1,
		LastAlive:       time.Now(),
	}
	DevMap.Set(devid, client)

	go func() {
		log.Tracef("start send routine for %s", conn.RemoteAddr().String())
		for {
			select {
			case pack := <-client.MsgOut:
				seqid := pack.client.NextSeqId
				pack.msg.Header.Seq = seqid
				b, _ := pack.msg.Header.Serialize()
				conn.Write(b)
				conn.Write(pack.msg.Data)
				log.Infof("send msg ok, (%s)", string(pack.msg.Data))
				pack.client.NextSeqId += 1
				// add reply channel
				if pack.reply != nil {
					pack.client.WaitingChannels[seqid] = pack.reply
				}
			case <-client.ctrl:
				log.Tracef("leave send routine for %s", conn.RemoteAddr().String())
				return
			}
		}
	}()
	return client
}

func CloseClient(client *Client) {
	client.ctrl <- true
	DevMap.Delete(client.devId)
}

func handleReply(client *Client, header *Header, body []byte) int {
	ch, ok := client.WaitingChannels[header.Seq]
	if ok {
		//remove waiting channel from map
		delete(client.WaitingChannels, header.Seq)
		ch <- &Message{Header: *header, Data: body}
	}
	return 0
}

func handleHeartbeat(client *Client, header *Header, body []byte) int {
	client.LastAlive = time.Now()
	return 0
}

func (this *Server) SetAcceptTimeout(timeout time.Duration) {
	this.acceptTimeout = timeout
}

func (this *Server) SetReadTimeout(timeout time.Duration) {
	this.readTimeout = timeout
}

func (this *Server) SetHeartbeatTimeout(timeout time.Duration) {
	this.heartbeatTimeout = timeout
}

func (this *Server) SetWriteTimeout(writeTimeout time.Duration) {
	this.writeTimeout = writeTimeout
}

func (this *Server) SetMaxPktLen(maxMsgLen uint32) {
	this.maxMsgLen = maxMsgLen
}

func (this *Server) Init(addr string) (*net.TCPListener, error) {
	tcpAddr, err := net.ResolveTCPAddr("tcp4", addr)
	l, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		log.Errorf("failed to listen, (%v)", err)
		return nil, err
	}
	this.funcMap[MSG_HEARTBEAT] = handleHeartbeat
	this.funcMap[MSG_REQUEST_REPLY] = handleReply
	return l, nil
}

func (this *Server) Run(listener *net.TCPListener) {
	this.waitGroup.Add(1)
	defer func() {
		listener.Close()
		this.waitGroup.Done()
	}()

	//go this.dealSpamConn()
	log.Infof("Starting comet server on: %s\n", listener.Addr().String())
	log.Infof("Comet server settings: readtimeout [%d], accepttimeout [%d], heartbeattimeout [%d]\n",
		this.readTimeout, this.acceptTimeout, this.heartbeatTimeout)
	for {
		select {
		case <-this.exitCh:
			log.Infof("Stopping comet server")
			return
		default:
		}

		listener.SetDeadline(time.Now().Add(this.acceptTimeout))
		conn, err := listener.AcceptTCP()
		if err != nil {
			if e, ok := err.(*net.OpError); ok && e.Timeout() {
				continue
			}
			log.Errorf("accept failed: %v\n", err)
			continue
		}
		/*
			// first packet must sent by client in specified seconds
			if err = conn.SetReadDeadline(time.Now().Add(20)); err != nil {
				glog.Errorf("conn.SetReadDeadLine() error(%v)", err)
				conn.Close()
				continue
			}*/
		go this.handleConnection(conn)
	}
}

func (this *Server) Stop() {
	// close后，所有的exitCh都返回false
	log.Infof("stopping comet server")
	close(this.exitCh)
	this.waitGroup.Wait()
	log.Infof("comet server stopped")
}

func waitRegister(conn *net.TCPConn) *Client {
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	buf := make([]byte, 10)
	n, err := io.ReadFull(conn, buf)
	if err != nil {
		log.Errorf("readfull header failed (%v)", err)
		conn.Close()
		return nil
	}

	var header Header
	if err := header.Deserialize(buf[0:n]); err != nil {
		log.Errorf("parse header (%v)", err)
		conn.Close()
		return nil
	}

	//log.Printf("body len %d", header.Len)
	data := make([]byte, header.Len)
	if _, err := io.ReadFull(conn, data); err != nil {
		log.Errorf("readfull body failed: (%v)", err)
		conn.Close()
		return nil
	}

	if header.Type != MSG_REGISTER {
		log.Warnf("not register message")
		conn.Close()
		return nil
	}

	devid := string(data)
	log.Infof("recv register devid (%s)", devid)
	if DevMap.Check(devid) {
		log.Warnf("device (%s) register already", devid)
		conn.Close()
		return nil
	}
	client := InitClient(conn, devid)
	return client
}

// handle a TCP connection
func (this *Server) handleConnection(conn *net.TCPConn) {
	log.Debugf("accept connection (%v)", conn)
	log.Infof("New conn accepted from %s\n", conn.RemoteAddr().String())
	// handle register first
	client := waitRegister(conn)
	if client == nil {
		return
	}

	for {
		/*
			select {
			case <- this.exitCh:
				log.Printf("ask me quit\n")
				return
			default:
			}
		*/

		now := time.Now()
		if now.After(client.LastAlive.Add(this.heartbeatTimeout)) {
			log.Warnf("heartbeat timeout")
			break
		}

		//conn.SetReadDeadline(time.Now().Add(this.readTimeout))
		conn.SetReadDeadline(now.Add(10 * time.Second))
		//headSize := 10
		buf := make([]byte, HEADER_SIZE)
		n, err := io.ReadFull(conn, buf)
		if err != nil {
			if e, ok := err.(*net.OpError); ok && e.Timeout() {
				//log.Printf("read timeout, %d", n)
				continue
			}
			log.Errorf("readfull failed (%v)", err)
			break
		}
		//log.Printf("read %d bytes", n)
		var header Header
		if err := header.Deserialize(buf[0:n]); err != nil {
			break
		}

		data := make([]byte, header.Len)
		if _, err := io.ReadFull(conn, data); err != nil {
			if e, ok := err.(*net.OpError); ok && e.Timeout() {
				continue
			}
			log.Errorf("read from client failed: (%v)", err)
			break
		}

		handler, ok := this.funcMap[header.Type]
		if ok {
			ret := handler(client, &header, data)
			if ret < 0 {
				break
			}
		}
	}
	// don't use defer to improve performance
	log.Infof("close connection %s\n", conn.RemoteAddr().String())
	CloseClient(client)
	conn.Close()
}
