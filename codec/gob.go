package codec

import (
	"bufio"
	"encoding/gob"
	"io"

	log "github.com/golang/glog"
)

type GobCodec struct {
	conn io.ReadWriteCloser
	buf  *bufio.Writer
	dec  *gob.Decoder
	enc  *gob.Encoder
}

func NewGobCodecFunc(conn io.ReadWriteCloser) Codec {
	buf := bufio.NewWriter(conn)
	return &GobCodec{
		conn: conn,
		buf:  buf,
		dec:  gob.NewDecoder(conn),
		enc:  gob.NewEncoder(buf),
	}
}

func (c *GobCodec) ReadReqHeader(h *ReqHeader) error {
	return c.dec.Decode(h)
}

func (c *GobCodec) ReadRspHeader(h *RspHeader) error {
	return c.dec.Decode(h)
}

func (c *GobCodec) ReadBody(body interface{}) error {
	return c.dec.Decode(body)
}

func (c *GobCodec) Write(h interface{}, body interface{}) (err error) {
	defer func() {
		_ = c.buf.Flush()
		if err != nil {
			log.Errorf("JsonCodec write err:%+v\n", err)
			_ = c.Close()
		}
	}()
	if err = c.enc.Encode(h); err != nil {
		return
	}
	if err = c.enc.Encode(body); err != nil {
		return
	}
	return
}

func (c *GobCodec) Close() error {
	return c.conn.Close()
}
