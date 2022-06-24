package connpool

import (
	"net"
	"syscall"
	"time"

	log "github.com/golang/glog"
)

// PoolConn 连接池中的连接
type PoolConn struct {
	net.Conn              // 通过匿名字段继承父类方法
	closed      bool      //连接是否已经关闭
	forceClose  bool      //是否强制关闭连接
	recycleTime time.Time //连接回收时间
	pool        *ConnPool //该连接所在的连接池
}

// Read 重写read函数,读失败时关闭连接
func (c *PoolConn) Read(b []byte) (n int, err error) {
	if c.closed {
		return 0, ErrConnClosed
	}
	n, err = c.Conn.Read(b)
	if err != nil {
		log.Errorf("addr:%s, conn read err:%+v\n", c.Conn.LocalAddr().String(), err)
		c.forceClose = true
		c.recycle()
	}
	return n, err
}

// Write 重写Write函数,写失败时关闭连接
func (c *PoolConn) Write(b []byte) (n int, err error) {
	if c.closed {
		return 0, ErrConnClosed
	}
	n, err = c.Conn.Write(b)
	if err != nil {
		log.Errorf("addr:%s, conn write err:%+v\n", c.Conn.LocalAddr().String(), err)
		c.forceClose = true
		c.recycle()
	}
	return n, err
}

// Close 重写Close函数
func (c *PoolConn) Close() error {
	c.recycle()
	return nil
}

// 回收连接
func (c *PoolConn) recycle() {
	if c == nil || c.closed {
		return
	}
	c.recycleTime = time.Now() //更新连接回收时间
	c.pool.lock.Lock()
	if c.forceClose {
		c.Conn.Close()
		c.closed = true
		c.pool.active--
	} else {
		c.pool.connQueue = append(c.pool.connQueue, c)
	}

	c.pool.lock.Unlock()
}

// 探测连接是否需要释放
func (c *PoolConn) needRelease() bool {
	// 空闲时间超过空闲间隔，释放连接
	if time.Now().Sub(c.recycleTime) > c.pool.expireInterval {
		return true
	}

	// 连接异常，释放连接
	if c.isConnErr() {
		return true
	}

	return false
}

// 探测空闲连接是否异常
func (c *PoolConn) isConnErr() bool {
	conn, _ := c.Conn.(syscall.Conn)
	rawConn, err := conn.SyscallConn()
	buf := make([]byte, 1)
	if err != nil {
		return true
	}

	var n int
	var readErr error
	err = rawConn.Read(func(fd uintptr) (done bool) {
		n, readErr = syscall.Read(int(fd), buf)
		return true

	})
	if err != nil {
		return true
	}
	// 连接已关闭
	if readErr == nil && n == 0 {
		return true
	}
	// 空闲连接读到数据，连接异常
	if n > 0 {
		return true
	}
	return false
}
