package comet

import (
	log "github.com/cihub/seelog"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"
	//"strings"
	"github.com/chenyf/gibbon/storage"
	"github.com/chenyf/gibbon/utils/safemap"
)

type MsgHandler func(*Client, *Header, []byte) int

type Server struct {
	Name              string // unique name of this server
	exitCh            chan bool
	waitGroup         *sync.WaitGroup
	funcMap           map[uint8]MsgHandler
	acceptTimeout     time.Duration
	readTimeout       time.Duration
	writeTimeout      time.Duration
	heartbeatInterval time.Duration
	heartbeatTimeout  time.Duration
	maxMsgLen         uint32
}

func NewServer() *Server {
	return &Server{
		Name:             "gibbon",
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
	DevId           string
	ctrl            chan bool
	MsgOut          chan *Message
	WaitingChannels map[uint32]chan *Message
	NextSeqId       uint32
	LastAlive       time.Time
	RegistTime      time.Time
}

func (this *Client) SendMessage(msgType uint8, body []byte, reply chan *Message) (seq uint32) {
	seq = this.NextSeq()
	header := Header{
		Type: msgType,
		Ver:  0,
		Seq:  seq,
		Len:  uint32(len(body)),
	}
	msg := &Message{
		Header: header,
		Data:   body,
	}

	// add reply channel
	if reply != nil {
		this.WaitingChannels[seq] = reply
	}
	this.MsgOut <- msg
	return seq
}

func (this *Client) MsgTimeout(seq uint32) {
	delete(this.WaitingChannels, seq)
}

func (this *Client) NextSeq() uint32 {
	return atomic.AddUint32(&this.NextSeqId, 1)
}

var (
	DevMap *safemap.SafeMap = safemap.NewSafeMap()
)

func (this *Server) InitClient(conn *net.TCPConn, devid string) *Client {
	// save the client device Id to storage
	if err := storage.Instance.AddDevice(this.Name, devid); err != nil {
		log.Infof("failed to put device %s into redis:", devid, err)
		return nil
	}

	client := &Client{
		DevId:           devid,
		ctrl:            make(chan bool),
		MsgOut:          make(chan *Message, 100),
		WaitingChannels: make(map[uint32]chan *Message),
		NextSeqId:       0,
		LastAlive:       time.Now(),
		RegistTime:      time.Now(),
	}
	DevMap.Set(devid, client)

	go func() {
		log.Tracef("start send routine for [%s] [%s]", devid, conn.RemoteAddr().String())
		for {
			select {
			case msg := <-client.MsgOut:
				b, _ := msg.Header.Serialize()
				conn.Write(b)
				conn.Write(msg.Data)
				log.Infof("send msg to [%s]. seq: %d. body: (%s)", devid, msg.Header.Seq, string(msg.Data))
			case <-client.ctrl:
				log.Tracef("leave send routine for [%s] [%s]", devid, conn.RemoteAddr().String())
				return
			}
		}
	}()
	return client
}

func (this *Server) CloseClient(client *Client) {
	client.ctrl <- true
	if err := storage.Instance.RemoveDevice(this.Name, client.DevId); err != nil {
		log.Errorf("failed to remove device %s from redis:", client.DevId, err)
	}
	DevMap.Delete(client.DevId)
}

func handleReply(client *Client, header *Header, body []byte) int {
	log.Debugf("Received reply from [%s]. seq: %d. body: (%s)", client.DevId, header.Seq, body)
	ch, ok := client.WaitingChannels[header.Seq]
	if ok {
		//remove waiting channel from map
		delete(client.WaitingChannels, header.Seq)
		ch <- &Message{Header: *header, Data: body}
	} else {
		log.Warnf("no waiting channel for seq: %d, device: %s", header.Seq, client.DevId)
	}
	return 0
}

func handleHeartbeat(client *Client, header *Header, body []byte) int {
	log.Debugf("Heartbeat from devid: %s", client.DevId)
	client.LastAlive = time.Now()
	return 0
}

func (this *Server) SetAcceptTimeout(timeout time.Duration) {
	this.acceptTimeout = timeout
}

func (this *Server) SetReadTimeout(timeout time.Duration) {
	this.readTimeout = timeout
}

func (this *Server) SetHeartbeatInterval(timeout time.Duration) {
	this.heartbeatInterval = timeout
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
	this.funcMap[MSG_ROUTER_COMMAND_REPLY] = handleReply

	if err := storage.Instance.InitDevices(this.Name); err != nil {
		log.Errorf("failed to InitDevices: %s", err.Error())
		return nil, err
	}

	// keep the data of this node not expired on redis
	go func() {
		for {
			select {
			case <-this.exitCh:
				log.Infof("exiting storage refreshing routine")
				return
			case <-time.After(10 * time.Second):
				storage.Instance.RefreshDevices(this.Name, 30)
			}
		}
	}()

	return l, nil
}

func (this *Server) Run(listener *net.TCPListener) {
	this.waitGroup.Add(1)
	defer func() {
		listener.Close()
		this.waitGroup.Done()
	}()

	//go this.dealSpamConn()
	log.Infof("Starting comet server on: %s", listener.Addr().String())
	log.Infof("Comet server settings: readtimeout [%dms], accepttimeout [%dms], heartbeatinterval [%dms] heartbeattimeout [%dms]",
		this.readTimeout/time.Millisecond,
		this.acceptTimeout/time.Millisecond,
		this.heartbeatInterval/time.Millisecond,
		this.heartbeatTimeout/time.Millisecond)

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
			log.Errorf("accept failed: %v", err)
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

func (this *Server) waitRegister(conn *net.TCPConn) *Client {
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
	client := this.InitClient(conn, devid)
	return client
}

// handle a TCP connection
func (this *Server) handleConnection(conn *net.TCPConn) {
	log.Infof("New conn accepted from %s", conn.RemoteAddr().String())
	// handle register first
	client := this.waitRegister(conn)
	if client == nil {
		return
	}

	var (
		readHeader = true
		bytesRead  = 0
		data       []byte
		header     Header
		startTime  time.Time
	)

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
			log.Warnf("Device [%s] heartbeat timeout", client.DevId)
			break
		}

		conn.SetReadDeadline(now.Add(10 * time.Second))
		if readHeader {
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
			if err := header.Deserialize(buf[0:n]); err != nil {
				log.Errorf("Deserialize header failed: %s", err.Error())
				break
			}

			if header.Len > MAX_BODY_LEN {
				log.Warnf("Msg body too big from device [%s]: %d", header.Len, client.DevId)
				break
			}

			if header.Len > 0 {
				data = make([]byte, header.Len)
				readHeader = false
				bytesRead = 0
				startTime = time.Now()
				continue
			}
		} else {
			n, err := conn.Read(data[bytesRead:])
			if err != nil {
				if e, ok := err.(*net.OpError); ok && e.Timeout() {
					if now.After(startTime.Add(this.readTimeout)) {
						log.Infof("read packet data timeout [%s]", client.DevId)
						break
					}
				} else {
					log.Infof("read from client [%s] failed: (%s)", client.DevId, err.Error())
					break
				}
			}
			if n > 0 {
				bytesRead += n
			}
			if uint32(bytesRead) < header.Len {
				continue
			}
			readHeader = true
			log.Debugf("%s: body (%s)", client.DevId, data)
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
	log.Infof("closing device [%s] [%s]", client.DevId, conn.RemoteAddr().String())
	this.CloseClient(client)
	log.Infof("close connection [%s]", conn.RemoteAddr().String())
	conn.Close()
}
