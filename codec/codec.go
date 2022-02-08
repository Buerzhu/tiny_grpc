package codec

import (
	"io"
)

const (
	JsonType = "application/json"
	PbType   = "application/x-protobuf"
)

type Header map[string]string

type Option struct {
	MagicNumber uint32
	Version     uint16
	CodecType   string
}

type ReqHeader struct {
	Seq           uint32
	ServiceMethod string
	CustomHeader  Header
}

type RspHeader struct {
	Seq          uint32
	Code         int
	Msg          string
	CustomHeader Header
}

type Codec interface {
	io.Closer
	ReadReqHeader(*Header) error
	ReadRspHeader(*Header) error
	ReadBody(interface{}) error
	Write(*Header, interface{}) error
}

type NewCodecFunc func(io.ReadWriteCloser) Codec
