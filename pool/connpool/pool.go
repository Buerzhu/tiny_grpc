package connpool

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"

	"github.com/Buerzhu/tiny_grpc/config"
	log "github.com/golang/glog"
	"golang.org/x/sync/singleflight"
)

var (
	ErrConnClosed = errors.New("conn closed")
	ErrNoDeadline = errors.New("ctx no deadline")
	ErrTimeout    = errors.New("context timeout")
)

var (
	defaultCapacity        = 65535                  //连接池默认大小
	defaultExpiredInterval = 5 * time.Second        //连接池空闲连接回收间隔时间， 5s
	defaultDialTimeout     = 200 * time.Millisecond //连接超时时间，200ms
)

type ConnPool struct {
	network        string
	capacity       int32         //连接数量上限
	expireInterval time.Duration //连接过期回收间隔
	dialTimeout    time.Duration //连接超时时间
	active         int32         //当前活跃连接数
	connQueue      []*PoolConn   //空闲连接队列
	close          bool          //当前连接池是否关闭
	lock           sync.Mutex    //互斥锁
	address        string        //服务端地址
	limit          bool          //是否限流
}

var (
	pools sync.Map //每个服务端地址对应一个连接池
	group singleflight.Group
)

// Get 从连接池中获取空闲连接
func Get(ctx context.Context, name string, address string, network string) (net.Conn, error) {
	var pool *ConnPool
	v, ok := pools.Load(address)
	if !ok {
		pool = initConnPool(name, address, network)
		conn, err := pool.getFreeConn(ctx)
		pools.Load(pool)
		return conn, err
	}
	pool = v.(*ConnPool)
	return pool.getFreeConn(ctx)
}

// 初始化连接池,name为路由名称，addr为ip地址
func initConnPool(name string, address string, network string) *ConnPool {
	pool, _, _ := group.Do(address, func() (interface{}, error) {
		conf := config.GetClientConfig(name).ConnPool
		cPool := &ConnPool{
			capacity:       int32(conf.Capacity),
			expireInterval: time.Duration(conf.ExpireInterval) * time.Millisecond,
			dialTimeout:    time.Duration(conf.Timeout) * time.Millisecond,
			limit:          conf.Limit,
			address:        address,
			network:        network,
		}
		cPool.releaseConnTiming()
		pools.Store(address, cPool)
		return cPool, nil
	})

	return pool.(*ConnPool)
}

// 获取空闲协程
func (p *ConnPool) getFreeConn(ctx context.Context) (*PoolConn, error) {
	waiting := false
	var item *PoolConn
	var err error

	p.lock.Lock()
	size := len(p.connQueue)
	if size != 0 {
		item = p.getConnFromQueue()
	} else if p.active < p.capacity {
		item, err = p.initConn(ctx)
		if err == nil {
			p.active = p.active + 1
		}
	} else {
		waiting = true
	}
	p.lock.Unlock()

	//若开启限流直接返回错误
	if waiting && p.limit {
		return nil, errors.New("conn pool is full,no free conn.")
	}

	//等待空闲连接
	if waiting {
		log.Infof("wait conn\n")
		//为防止死锁，进入循环探测
		for {
			// 超时不等待空闲连接，直接返回
			if ctx.Err() != nil {
				return nil, ErrTimeout
			}
			p.lock.Lock()
			size := len(p.connQueue)
			if size > 0 {
				item = p.getConnFromQueue()
				waiting = false
			}
			p.lock.Unlock()
			if !waiting {
				break
			}
		}
	}
	return item, err
}

func (p *ConnPool) initConn(ctx context.Context) (*PoolConn, error) {
	if err := ctx.Err(); err != nil {
		log.Errorf("initConn fail because err:%+v\n", err)
		return nil, ctx.Err()
	}

	timeout := p.dialTimeout
	d, ok := ctx.Deadline()
	if ok {
		t := time.Until(d)
		if t < timeout {
			timeout = t
		}
	}
	conn, err := net.DialTimeout(p.network, p.address, timeout)
	if err != nil {
		return nil, err
	}
	return &PoolConn{
		Conn: conn,
		pool: p,
	}, nil
}

func (p *ConnPool) getConnFromQueue() *PoolConn {
	size := len(p.connQueue)
	if size == 0 {
		return nil
	}
	conn := p.connQueue[size-1]
	p.connQueue[size-1] = nil
	p.connQueue = p.connQueue[:(size - 1)]
	return conn
}

// releaseConnTiming 定时释放空闲连接
func (p *ConnPool) releaseConnTiming() {
	go func() {
		//创建一个周期性的定时器
		ticker := time.NewTicker(p.expireInterval)
		for range ticker.C {
			if p.close {
				return
			}

			p.lock.Lock()
			if len(p.connQueue) <= 0 {
				p.lock.Unlock()
				continue
			}

			index := 0
			for ; index < len(p.connQueue); index = index + 1 {
				item := p.connQueue[index]
				// 先进先出原则，优先回收队列前面的连接
				if !item.needRelease() {
					p.connQueue = p.connQueue[index:]
					log.Infof("start release conn.release conn count:%d\n", index)
					break
				}
				item.Conn.Close()
				item.closed = true
				p.active--
				p.connQueue[index] = nil
			}
			if index < len(p.connQueue) {
				p.connQueue = p.connQueue[index:] //在连接队列中删除无效连接
			} else {
				p.connQueue = p.connQueue[:0]
			}
			p.lock.Unlock()
		}
	}()
}
