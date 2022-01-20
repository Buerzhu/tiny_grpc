package codec

import (
	"io"
)

const (
	JsonType = "application/json"
	PbType   = "application/x-protobuf"
)

type Option struct {
	MagicNumber uint32
	Version     uint16
	CodecType   string
}

type Header struct {
	MsgID     string
	Target    string
	ErrorMsg  string
	ErrorCode uint32
}

type Codec interface {
	io.Closer
	ReadHeader(*Header) error
	ReadBody(interface{}) error
	Write(*Header, interface{}) error
}

type NewCodecFunc func(io.ReadWriteCloser) Codec
