package comet

import (
	"bytes"
	"encoding/binary"
)

type Header struct {
	Type uint8
	Ver  uint8
	Seq  uint32
	Len  uint32
}

type Message struct {
	Header Header
	Data   []byte
}

const (
	HEADER_SIZE  = 10
	MAX_BODY_LEN = 20 * 1024
)

const (
	MSG_HEARTBEAT      = uint8(0)
	MSG_REGISTER       = uint8(1)
	MSG_REGISTER_REPLY = uint8(2)
	MSG_REQUEST        = uint8(3)
	MSG_REQUEST_REPLY  = uint8(4)
	MSG_ECHO           = uint8(8)
	MSG_ECHO_REPLY     = uint8(9)
	MSG_COMMAND        = uint8(10)
)

// msg to byte
func (header *Header) Serialize() ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := binary.Write(buf, binary.BigEndian, *header); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// byte to msg
func (header *Header) Deserialize(b []byte) error {
	buf := bytes.NewReader(b)
	if err := binary.Read(buf, binary.BigEndian, header); err != nil {
		return err
	}
	return nil
}

type RegisterMessage struct {
}

