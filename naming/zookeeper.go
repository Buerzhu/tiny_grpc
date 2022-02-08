package naming

import (
	"fmt"
	"sync"
	"time"

	"github.com/Buerzhu/tiny_grpc/config"
	log "github.com/golang/glog"
	"github.com/samuel/go-zookeeper/zk"
	"golang.org/x/sync/singleflight"
)

type client struct {
	conn *zk.Conn
	addr []string
}

var (
	once     sync.Once
	g        singleflight.Group
	zkClient *client
)

func Register(tcpAddr string) error {
	c := getZKClient()
	if c == nil {
		return fmt.Errorf("conn zookeeper fail")
	}

	//创建根节点
	err := c.createNode(config.GetBusinessName())
	if err != nil {
		return err
	}

	//创建子节点
	err = c.createNode(config.GetBusinessName() + config.GetServiceName())
	if err != nil {
		return err
	}

	err = c.createTemporaryNode(config.GetBusinessName()+config.GetServiceName()+"/node", []byte(tcpAddr))
	log.Info("Register suc")
	return err
}

func getZKClient() *client {
	once.Do(func() {
		addr := config.GetRegistrationAddr()

		// 连接zookeeper
		conn, _, err := zk.Connect(addr, time.Duration(5)*time.Second)
		if err != nil {
			log.Errorf("conn zookeeper fail because err: %+v", err)
			return
		}

		zkClient = &client{
			addr: addr,
			conn: conn,
		}
	})
	return zkClient
}

// 创建持久节点
func (c *client) createNode(path string) error {
	exists, _, err := c.conn.Exists(path)
	if err != nil {
		return err
	}
	if !exists {
		_, err, _ := g.Do(path, func() (interface{}, error) {
			_, err := c.conn.Create(path, []byte(""), 0, zk.WorldACL(zk.PermAll))
			if err != nil && err != zk.ErrNodeExists {
				return nil, err
			}
			return nil, nil
		})
		if err != nil {
			return err
		}
	}

	return nil
}

// 创建保护顺序临时子节点，会话关闭后节点自动消失
func (c *client) createTemporaryNode(path string, data []byte) error {
	_, err := c.conn.CreateProtectedEphemeralSequential(path, data, zk.WorldACL(zk.PermAll))
	return err
}

// 关闭连接，释放临时节点
func (s *client) Close() {
	s.conn.Close()
}

func GetNodes(path string) ([]string, error) {
	s := getZKClient()
	if s == nil {
		return nil, fmt.Errorf("GetNodes fail because conn zookeeper fail")
	}

	// 获取子节点名称
	childs, _, err := s.conn.Children(path)
	if err != nil {
		if err == zk.ErrNoNode {
			return nil, zk.ErrNoNode
		}
		log.Errorf("get son node fail because err:%+v, path:%s\n", err, path)
		return nil, err
	}

	nodes := []string{}
	for _, child := range childs {
		fullPath := path + "/" + child
		data, _, err := s.conn.Get(fullPath)
		if err != nil {
			if err == zk.ErrNoNode {
				continue
			}
			log.Errorf("path %s get son node fail because err:%+v", fullPath, err)
			return nil, err
		}
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, string(data))
	}
	if len(nodes) == 0 {
		return nil, fmt.Errorf("GetNodes fail because service %s has no node", path)
	}
	return nodes, nil
}
