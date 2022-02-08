package codec

import (
	"bufio"
	"encoding/json"
	"io"
	"log"
)

type JsonCodec struct {
	conn io.ReadWriteCloser
	buf  *bufio.Writer
	dec  *json.Decoder
	enc  *json.Encoder
}

func NewJsonCodecFunc(conn io.ReadWriteCloser) *JsonCodec {
	buf := bufio.NewWriter(conn)
	return &JsonCodec{
		conn: conn,
		buf:  buf,
		dec:  json.NewDecoder(conn),
		enc:  json.NewEncoder(buf),
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

func (c *JsonCodec) Write(h *Header, body interface{}) (err error) {
	defer func() {
		_ = c.buf.Flush()
		if err != nil {
			_ = c.conn.Close()
		}
	}()
	err = c.enc.Encode(h)
	if err != nil {
		log.Printf("JsonCodec write header fail. header:%+v, err:%+v", h, err)
		return
	}

	err = c.enc.Encode(body)
	if err != nil {
		log.Printf("JsonCodec write body fail. body:%+v, err:%+v", body, err)
		return
	}

	return
}
