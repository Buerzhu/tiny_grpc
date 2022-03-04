package pool

import (
	"net"
	"sync"
)

type connPool struct {
	dialErrorsNum uint32 // atomic

	lastDialErrorMu sync.RWMutex
	lastDialError   error

	queue chan struct{}

	mc           sync.Mutex
	conns        []*netConn
	idleConns    []*netConn
	poolSize     int32
	idleConnsLen int32

	_closed uint32 // atomic
}

type netConn struct {
	conn net.Conn
}

func reTEst() {
}
