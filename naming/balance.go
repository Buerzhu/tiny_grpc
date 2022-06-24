package naming

import (
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/Buerzhu/tiny_grpc/config"
	log "github.com/golang/glog"
	"golang.org/x/sync/singleflight"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

//服务端节点地址信息缓存
var syncMap sync.Map

var group singleflight.Group

//服务端节点信息
type roundBalance struct {
	servicePath string    //服务名称,格式为:/业务名/服务名
	nodeAddrs   []string  //节点列表
	curIndex    int       //当前节点索引
	expiredTime time.Time //缓存过期时间
}

// 更新节点信息
func updateNodeInfo(servicePath string) (*roundBalance, error) {
	node, err, _ := group.Do(servicePath, func() (interface{}, error) {
		addrs, err := GetNodes(servicePath)
		if err != nil {
			return nil, fmt.Errorf("updateNodeInfo fail because of GetNodes err:%+v", err)
		}
		if len(addrs) == 0 {
			return nil, errors.New("updateNodeInfo fail because of zk return empty addr list")
		}
		expire := time.Duration(config.GetRegistrationConfig().Expire) * time.Millisecond
		balance := &roundBalance{
			servicePath: servicePath,
			nodeAddrs:   addrs,
			expiredTime: time.Now().Add(expire),
		}
		syncMap.Store(servicePath, balance)
		return balance, nil
	})
	if err != nil {
		return nil, err
	}

	return node.(*roundBalance), err
}

// 根据服务名获取服务实际的ip:port
func GetNodeAddr(servicePath string) (string, error) {
	val, _ := syncMap.Load(servicePath)

	//若有缓存则从缓存获取信息
	if val != nil {
		v, _ := val.(*roundBalance)
		//缓存过期则更新缓存
		if time.Now().Sub(v.expiredTime) > 0 {
			go func() {
				_, err := updateNodeInfo(servicePath)
				if err != nil {
					log.Error(err)
				}
			}()
		}
		curIndex := rand.Intn(1000) % len(v.nodeAddrs) //随机负载均衡
		return v.nodeAddrs[curIndex], nil
	}

	//无缓存情况下请求zookeeper来初始化缓存
	node, err := updateNodeInfo(servicePath)
	if err != nil {
		return "", fmt.Errorf("Service %s getNodeAddr fail because updateNodeAddr err:%+v", servicePath, err)
	}
	return node.nodeAddrs[0], nil
}
