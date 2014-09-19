package comet

import (
	"log"
	"io"
	//"bufio"
	"net"
	"sync"
	"time"
	//"strconv"
	//"strings"
	//"github.com/chenyf/gibbon/utils/convert"
	"github.com/chenyf/gibbon/utils/safemap"
)

type Server struct {
	exitCh         chan bool
	waitGroup      *sync.WaitGroup
	funcMap        map[uint8]func(*net.TCPConn, Header, []byte)(int)
	acceptTimeout  time.Duration
	readTimeout    time.Duration
	writeTimeout   time.Duration
	maxMsgLen      uint32
}

func NewServer() *Server {
	return &Server {
		exitCh:        make(chan bool),
		waitGroup:     &sync.WaitGroup{},
		funcMap:       make(map[uint8]func(*net.TCPConn, Header, []byte)(int)),
		acceptTimeout: 60,
		readTimeout:   60,
		writeTimeout:  60,
		maxMsgLen:     2048,
	}
}

type Client struct {
	devId	string
	ctrl	chan bool
	MsgOut	chan Message
	MsgFoo	map[uint32]chan Message
	NextSeqId uint32
}

func (client *Client)SendMessage(msgType uint8, body []byte, wait bool) (chan Message, uint32) {
	seqid := client.NextSeqId
	header := Header{
		Type:	msgType,
		Ver:	0,
		Seq:	seqid,
		Len:	uint32(len(body)),
	}
	msg := Message{
		Header: header,
		Data:	body,
	}
	client.NextSeqId += 1
	if !wait {
		client.MsgOut <- msg
		return nil, 0
	}
	ch := make(chan Message)
	client.MsgFoo[seqid] = ch
	client.MsgOut <- msg
	return ch, seqid

}

var (
	DevMap *safemap.SafeMap = safemap.NewSafeMap()
	ConnMap *safemap.SafeMap = safemap.NewSafeMap()
)

func InitClient(conn *net.TCPConn, devid string) {
	ConnMap.Set(conn, devid)
	client := &Client {
		devId: devid,
		ctrl: make(chan bool),
		MsgOut: make(chan Message, 100),
		MsgFoo: make(map[uint32]chan Message),
		NextSeqId: 1,
	}
	DevMap.Set(devid, client)

	go func() {
		log.Printf("enter send routine")
		for {
			select {
			case msg := <-client.MsgOut:
				b, _ := msg.Header.Serialize()
				conn.Write(b)
				conn.Write(msg.Data)
				log.Printf("send msg ok, (%s)", string(msg.Data))
			case <-client.ctrl:
				break
			}
		}
		log.Printf("leave send routine")
	}()
}

func CloseClient(conn *net.TCPConn) {
	conn.Close()
	devid := ConnMap.Get(conn)
	DevMap.Delete(devid)
	ConnMap.Delete(conn)
}

func handleReply(conn *net.TCPConn, header Header, body []byte) int {
	devid := ConnMap.Get(conn)
	client, _ := DevMap.Get(devid).(*Client)
	seqid := header.Seq
	ch, ok := client.MsgFoo[seqid]; if ok {
		ch <- Message{Header: header, Data: body}
	}
	return 0
}

// type seq ver len body
func handleRegister(conn *net.TCPConn, header Header, body []byte) int {
	devid := string(body)
	log.Printf("recv register devid (%s)", devid)
	/*a := strings.Split(body, " ")
	if len(a) != 4 {
		return false
	}*/
	InitClient(conn, devid)
	return 0
}


func (this *Server) SetAcceptTimeout(acceptTimeout time.Duration) {
	this.acceptTimeout = acceptTimeout
}

func (this *Server) SetReadTimeout(readTimeout time.Duration) {
	this.readTimeout = readTimeout
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
		log.Printf("failed to listen, (%v)", err)
		return nil, err
	}
	this.funcMap[MSG_REGISTER] = handleRegister
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
	log.Printf("comet server start\n")
	for {
		select {
		case <- this.exitCh:
			log.Printf("ask me to quit")
			return
		default:
		}

		listener.SetDeadline(time.Now().Add(2*time.Second))
		//listener.SetDeadline(time.Now().Add(this.acceptTimeout))
		//log.Printf("before accept, %d", this.acceptTimeout)
		conn, err := listener.AcceptTCP()
		//log.Printf("after accept")
		if err != nil {
			if e, ok := err.(*net.OpError); ok && e.Timeout() {
				//log.Printf("accept timeout")
				continue
			}
			log.Printf("accept failed: %v\n", err)
			continue
		}
		log.Printf("accept new connection\n")
		go this.handleConnection(conn)
	}
}

func (this *Server) Stop() {
	// close后，所有的exitCh都返回false
	log.Printf("stopping comet server")
	close(this.exitCh)
	this.waitGroup.Wait()
	log.Printf("comet server stopped")
}

/// this routine used for receive message
func (this *Server)handleConnection(conn *net.TCPConn) {

	// recv data
	/*
	var (
		bLen []byte = make([]byte, 4)
		bType []byte = make([]byte, 4)
		msgLen uint32
	)*/

	defer func() {
		log.Printf("close connection")
		CloseClient(conn)
	}()

	for {
		/*
		select {
		case <- this.exitCh:
			log.Printf("ask me quit\n")
			return
		default:
		}
		*/

		//conn.SetReadDeadline(time.Now().Add(this.readTimeout))
		/*
		if n, err := io.ReadFull(conn, bLen); err != nil  && n != 4 {
			log.Printf("read message len failed, (%v)\n", err)
			return
		}

		if n, err := io.ReadFull(conn, bType); err != nil && n != 4 {
			log.Printf("read message type failed\n")
			return
		}

		if msgLen = convert.BytesToUint32(bLen); msgLen > this.maxMsgLen {
			log.Printf("msg too long %d\n", msgLen)
			return
		}

		bData := make([]byte, msgLen-8)
		if n, err := io.ReadFull(conn, bData); err != nil && n != int(msgLen) {
			return
		}

		*/
		//conn.SetReadDeadline(time.Now().Add(this.readTimeout))
		conn.SetReadDeadline(time.Now().Add(10* time.Second))
		//headSize := 10
		buf := make([]byte, 10)
		n, err := io.ReadFull(conn, buf)
		if err != nil {
			if e, ok := err.(*net.OpError); ok && e.Timeout() {
				//log.Printf("read timeout")
				continue
			}
		}
		log.Printf("read %d bytes", n)
		var header Header
		if err := header.Deserialize(buf[0:n]); err != nil {
			return
		}

		data := make([]byte, header.Len)
		if _, err := io.ReadFull(conn, data); err != nil {
			if e, ok := err.(*net.OpError); ok && e.Timeout() {
				continue
			}
			log.Printf("read from client failed: (%v)", err)
			return
		}

		if header.Type != MSG_REGISTER {
			if !ConnMap.Check(conn) {
				log.Printf("hasn't registered")
				continue
			}
		} else {
			devid := string(data)
			if DevMap.Check(devid) {
				log.Printf("device (%s) register already", devid)
				return
			}
		}
		handler, ok := this.funcMap[header.Type]; if ok {
			handler(conn, header, data)
			//log.Printf("call result %d", ret)
		} else {
			log.Printf("unknown message type, %d\n", header.Type)
		}
	}
}


