package client

import (
	"time"

	"github.com/Buerzhu/tiny_grpc/codec"
)

type Option func(*codec.Option)

// WithTimeout 设置超时时间
func WithTimeout(timeout time.Duration) Option {
	return func(o *codec.Option) {
		o.Timeout = timeout
	}
}

// WithSerialization 设置序列化方式
func WithSerialization(typ string) Option {
	return func(o *codec.Option) {
		o.CodecType = typ
	}
}

// WithKeepAlive 设置连接方式
func WithKeepAlive(keepAlive bool) Option {
	return func(o *codec.Option) {
		o.KeepAlive = keepAlive
	}
}
