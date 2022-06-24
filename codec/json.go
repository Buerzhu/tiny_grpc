package codec

import (
	"bufio"
	"encoding/json"
	"io"

	log "github.com/golang/glog"
)

type JsonCodec struct {
	conn io.ReadWriteCloser
	buf  *bufio.Writer
	dec  *json.Decoder
	enc  *json.Encoder
}

func NewJsonCodecFunc(conn io.ReadWriteCloser) Codec {
	// 使用buf提高写效率
	buf := bufio.NewWriter(conn)
	return &JsonCodec{
		conn: conn,
		buf:  buf,
		dec:  json.NewDecoder(conn),
		enc:  json.NewEncoder(conn),
	}

}

func (c *JsonCodec) ReadReqHeader(h *ReqHeader) error {
	return c.dec.Decode(h)
}
func (c *JsonCodec) ReadRspHeader(h *RspHeader) error {
	return c.dec.Decode(h)
}

func (c *JsonCodec) ReadBody(body interface{}) error {
	return c.dec.Decode(body)
}

func (c *JsonCodec) Write(h interface{}, body interface{}) (err error) {
	defer func() {
		_ = c.buf.Flush()
		if err != nil {
			log.Errorf("JsonCodec write err:%+v\n", err)
			_ = c.conn.Close()
		}
	}()

	err = c.enc.Encode(h)
	if err != nil {
		log.Errorf("JsonCodec write header fail. header:%+v, err:%+v", h, err)
		return
	}

	err = c.enc.Encode(body)
	if err != nil {
		log.Errorf("JsonCodec write body fail. body:%+v, err:%+v", body, err)
		return
	}

	return
}

func (c *JsonCodec) Close() error {
	return c.conn.Close()
}
