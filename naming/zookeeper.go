package naming

import (
	"errors"
	"fmt"
	"strings"
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

var (
	serviceName  string
	businessName string
	serverName   string
	splitOnce    sync.Once
)

// name，格式为：业务名/服务名
func Register(name string, addr string) error {
	service := getServiceName()
	business := getBusinessName()
	if service == "" || business == "" {
		return errors.New("invalid server name")
	}
	// 注册的服务名必须要与框架服务名一致
	if name != serverName {
		return fmt.Errorf("server name not equal config server name. name:%s, server:%s",
			name, serverName)
	}
	c := getZKClient()
	if c == nil {
		return fmt.Errorf("conn zookeeper fail")
	}

	//创建根节点
	err := c.createNode("/" + business)
	if err != nil {
		return err
	}

	//创建子节点
	path := "/" + business + "/" + service
	err = c.createNode(path)
	if err != nil {
		return err
	}

	// 生成临时节点，当服务与zookeeper的连接断开时会在zk上删除该节点信息
	err = c.createTemporaryNode(path+"/node", []byte(addr))
	if err == nil {
		log.Infof("Register suc. server:%s, addr:%s", path, addr)
	}
	return err
}

func getZKClient() *client {
	if zkClient != nil {
		return zkClient
	}
	once.Do(func() {
		conf := config.GetRegistrationConfig()

		// 连接zookeeper
		conn, _, err := zk.Connect([]string{conf.Addr}, time.Duration(conf.Timeout)*time.Millisecond)
		if err != nil {
			log.Errorf("conn zookeeper fail because err: %+v", err)
			return
		}

		zkClient = &client{
			addr: []string{conf.Addr},
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

	return err
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

// 根据服务名获取服务网络地址列表
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
			log.Errorf("path %s get son node fail because err:%+v\n", fullPath, err)
			return nil, err
		}
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, string(data))
	}
	if len(nodes) == 0 {
		return nil, fmt.Errorf("GetNodes fail because service %s has no node\n", path)
	}
	return nodes, nil
}

// 从配置文件中获取服务名
func getServiceName() string {
	if serviceName != "" {
		return serviceName
	}

	splitOnce.Do(func() {
		serverConf := config.GetServerConfig()
		serverName = serverConf.Name
		splitServiceAndBusiness(serverConf.Name)
	})

	return serviceName
}

// 从配置文件中获取业务名
func getBusinessName() string {
	if businessName != "" {
		return businessName
	}

	splitOnce.Do(func() {
		serverConf := config.GetServerConfig()
		serverName = serverConf.Name
		splitServiceAndBusiness(serverConf.Name)
	})

	return businessName
}

func splitServiceAndBusiness(name string) error {
	index := strings.Index(name, "/")
	if index == -1 || index == 0 || index >= (len(name)-1) {
		log.Errorf("splitServiceAndBusiness fail. Invaild name: %s", name)
		return errors.New("invalid name")
	}

	businessName = name[:index]
	serviceName = name[(index + 1):]
	return nil
}
