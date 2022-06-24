package codec

import (
	"errors"
	"io"
	"time"
)

const (
	JsonType = "json"
	PbType   = "pb"
	GobType  = "gob"
)

const (
	TraceID   = "traceID"
	CustomMsg = "customMsg"
)

type Header map[string]interface{}

const MagicNumber = 0x6a7b8c9e
const Version = 1

// Option 传递rpc通用参数
type Option struct {
	MagicNumber uint32        // 魔数，用于标识是否为rpc请求
	Version     uint8         // rpc框架版本信息
	KeepAlive   bool          // 标识是否为长连接
	CodecType   string        // 最多有15种序列化类型
	Timeout     time.Duration // 最大值为65535ms
}

// ReqHeader rpc请求头
type ReqHeader struct {
	ID            string // 标识请求的唯一id
	ServiceMethod string // 传递服务名和接口名，格式：服务名/接口名
	CustomHeader  Header //用户自定义头信息
}

// RspHeader rpc响应头
type RspHeader struct {
	ID           string // 标识请求的唯一id
	Code         int    // 返回码，非零为异常
	Msg          string // 返回信息，对返回码的说明
	CustomHeader Header // 服务端自定义头信息
}

type Codec interface {
	io.Closer
	ReadReqHeader(*ReqHeader) error
	ReadRspHeader(*RspHeader) error
	ReadBody(interface{}) error
	Write(interface{}, interface{}) error
}

type NewCodecFunc func(io.ReadWriteCloser) Codec

var CodecMap = make(map[string]NewCodecFunc)
var CodecToIntMap = make(map[string]uint8)
var CodecToStringMap = make(map[uint8]string)

func init() {
	CodecMap = make(map[string]NewCodecFunc)
	CodecMap[JsonType] = NewJsonCodecFunc
	CodecMap[GobType] = NewGobCodecFunc
	CodecToIntMap[JsonType] = 0
	CodecToIntMap[GobType] = 1
	CodecToStringMap[0] = JsonType
	CodecToStringMap[1] = GobType
}

// 根据类型返回对应的序列化方法
func GetCodec(typ string, conn io.ReadWriteCloser) (Codec, error) {
	v, ok := CodecMap[typ]
	if !ok {
		return nil, errors.New("invaild codec type")
	}
	return v(conn), nil
}

// DefaultOption 默认配置项
func DefaultOption() *Option {
	return &Option{
		MagicNumber: MagicNumber,
		Version:     0,
		CodecType:   JsonType,
		Timeout:     1000,
	}
}
