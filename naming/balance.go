package naming

import (
	"fmt"
	"sync"
	"time"

	"github.com/Buerzhu/tiny_grpc/config"
	log "github.com/golang/glog"
)

//服务端节点地址信息缓存
var syncMap sync.Map

//服务端节点信息,使用round-robin轮询节点
type roundBalance struct {
	serviceName string   //服务名称
	nodeAddrs   []string //节点列表
	curIndex    int      //当前节点索引
	expiredTime int64    //缓存过期时间
}

func updateNodeInfo(v *roundBalance) error {
	log.Infof("service %s start updateNodeInfo", v.serviceName)
	addrs, err := GetNodes(v.serviceName)
	if err != nil {
		return err
	}
	v.nodeAddrs = addrs
	v.expiredTime = time.Now().Unix() + config.GetAddrCacheExpiredTime()
	v.curIndex = (v.curIndex) % len(addrs)

	return nil
}

func GetNodeAddr(serviceName string) (string, error) {
	val, _ := syncMap.Load(serviceName)

	//若有缓存则从缓存获取信息
	if val != nil {
		v, _ := val.(*roundBalance)
		//缓存过期则更新缓存
		if v.expiredTime < time.Now().Unix() {
			err := updateNodeInfo(v)
			if err != nil {
				return "", err
			}
		}
		v.curIndex = (v.curIndex + 1) % len(v.nodeAddrs)
		return v.nodeAddrs[v.curIndex], nil
	}

	//无缓存情况下请求zookeeper来初始化缓存
	log.Infof("service %s start init cache", serviceName)
	node := &roundBalance{
		serviceName: serviceName,
	}
	err := updateNodeInfo(node)
	if err != nil {
		return "", fmt.Errorf("GetNodeAddr fail because updateNodeAddr err:%+v", err)
	}
	syncMap.Store(serviceName, node)

	return node.nodeAddrs[0], nil
}
