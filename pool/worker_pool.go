package pool

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Buerzhu/tiny_grpc/config"
	log "github.com/golang/glog"
)

var (
	wOnce sync.Once
	wpool *workerPool
)

type sig struct{}
type f func() error

type workerPool struct {
	capacity       int32      //协程数量上限
	sig            chan sig   //用于接受关闭协程信号
	expireInterval int64      //协程过期回收间隔
	active         int32      //当前活跃协程数
	workerQueue    []*worker  //协程队列
	close          bool       //当前协程池是否关闭
	lock           sync.Mutex //互斥锁
}

type worker struct {
	pool        *workerPool //该协程所在的协程
	recycleTime int64       //协程上次活跃时间,unix时间戳，单位s
	taskChan    chan f      //任务通道，用于执行外部传入的任务
}

// 提交任务到协程池
func SubmitTask(task f) error {
	workerPool := initWorkerPool()
	if workerPool.close {
		return errors.New("SubmitTask fail because worker pool had been closed")
	}

	worker := workerPool.getFreeWorker()
	if worker == nil {
		return errors.New("SubmitTask fail because worker pool return empty worker")
	}
	worker.taskChan <- task //将任务传入协程运行
	return nil
}

// 初始化协程池
func initWorkerPool() *workerPool {
	if wpool != nil {
		return wpool
	}

	wOnce.Do(func() {
		wpool = &workerPool{
			capacity:       config.GetWPoolCap(),
			expireInterval: config.GetAddrCacheExpiredTime(),
			sig:            make(chan sig, 1),
		}
	})
	return wpool
}

// 获取空闲协程
func (w *workerPool) getFreeWorker() *worker {
	waiting := false
	var item *worker
	w.lock.Lock()
	size := len(w.workerQueue)
	if size != 0 {
		item = w.getWorkerFromQueue()
	} else if w.active < w.capacity {
		item = &worker{
			pool:     w,
			taskChan: make(chan f, 1),
		}
		item.run()
		w.active = w.active + 1
	} else {
		waiting = true
	}
	w.lock.Unlock()

	//等待空闲协程
	if waiting {
		//为防止死锁，进入循环探测
		for {
			w.lock.Lock()
			size := len(w.workerQueue)
			if size > 0 {
				item = w.getWorkerFromQueue()
				waiting = false
			}
			w.lock.Unlock()
			if !waiting {
				break
			}
		}
	}
	return item
}

func (w *workerPool) getWorkerFromQueue() *worker {
	size := len(w.workerQueue)
	if size == 0 {
		return nil
	}
	worker := w.workerQueue[size-1]
	w.workerQueue[size-1] = nil
	w.workerQueue = w.workerQueue[:(size - 1)]
	return worker
}

// releaseWorkerTiming 定时释放空闲协程
func (w *workerPool) releaseWorkerTiming() {
	go func() {
		//创建一个周期性的定时器
		ticker := time.NewTicker(time.Duration(w.expireInterval) * time.Second)
		for range ticker.C {
			if w.close {
				return
			}

			now := time.Now()
			w.lock.Lock()
			if len(w.workerQueue) <= 0 {
				continue
			}
			for index := 0; index < len(w.workerQueue); index = index + 1 {
				item := w.workerQueue[index]
				if item.recycleTime > (now.Unix() - w.expireInterval) {
					w.workerQueue = w.workerQueue[index:]
					log.Infof("%+v: start release worker.release count:%d\n", now, index)
					break
				}
				item.taskChan <- nil
				w.workerQueue[index] = nil
				if index == len(w.workerQueue)-1 {
					w.workerQueue = w.workerQueue[:0]
					log.Infof("%+v: start release worker.release count:%d\n", now, index)
				}
			}
			w.lock.Unlock()
		}
	}()
}

// 协程执行外部传入的任务
func (w *worker) run() {
	w.pool.incRunning()
	go func() {
		for f := range w.taskChan {
			if w.pool.close {
				break
			}
			if f == nil {
				w.pool.decRunning()
				break
			} else {
				f()
				w.recycle() //执行完任务，将当前协程放入空闲协程队列
			}
		}
	}()
}

// 回收协程
func (w *worker) recycle() {
	w.recycleTime = time.Now().Unix() //更新协程回收时间
	w.pool.lock.Lock()
	w.pool.workerQueue = append(w.pool.workerQueue, w)
	w.pool.lock.Unlock()
}

// 协程池的扩缩容
func (w *workerPool) resize(size int32) {
	if size == w.capacity {
		return
	}

	atomic.StoreInt32(&w.capacity, size)
	if size < w.active {
		diff := w.active - size
		for i := 0; i < int(diff); i++ {
			w.getFreeWorker().taskChan <- nil
		}
	}
}

func (w *workerPool) decRunning() {
	atomic.AddInt32(&w.active, -1)
}
func (w *workerPool) incRunning() {
	atomic.AddInt32(&w.active, 1)
}
