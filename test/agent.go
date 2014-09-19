package main

import (
	"net"
	"log"
	"io"
	"os"
	//"fmt"
	//"strings"
	"github.com/chenyf/gibbon/comet"
)

func main() {

	if len(os.Args) <= 2 {
		log.Printf("Usage: server_addr devid")
		return
	}
	svraddr := os.Args[1]
	devid := os.Args[2]

	addr, _ := net.ResolveTCPAddr("tcp4", svraddr)
	conn, err := net.DialTCP("tcp", nil, addr)
	if err != nil {
		log.Printf("connect to server failed: %v", err)
		return
	}
	conn.SetNoDelay(true)
	defer conn.Close()

	header := comet.Header{
		Type: comet.MSG_REGISTER,
		Ver: 0,
		Seq: 0,
		Len: uint32(len(devid)),
	}

	b, _ := header.Serialize()
	n1, _ := conn.Write(b)
	n2, _ := conn.Write([]byte(devid))
	log.Printf("write out %d, %d", n1, n2)

	for {
		headSize := 10
		buf := make([]byte, headSize)
		if _, err := io.ReadFull(conn, buf); err != nil {
			log.Printf("read message len failed, (%v)\n", err)
			return
		}

		var header comet.Header
		if err := header.Deserialize(buf); err != nil {
			return
		}

		data := make([]byte, header.Len)
		if _, err := io.ReadFull(conn, data); err != nil {
			log.Printf("read from client failed: (%v)", err)
			return
		}

		log.Printf("recv from server (%s)", string(data))
		if header.Type == comet.MSG_REQUEST {
			s := "haha"
			reply_header := comet.Header{
				Type: comet.MSG_REQUEST_REPLY,
				Ver: 0,
				Seq: header.Seq,
				Len: uint32(len(s)),
			}
			b, _ := reply_header.Serialize()
			conn.Write(b)
			conn.Write([]byte(s))
		}
	}
}


