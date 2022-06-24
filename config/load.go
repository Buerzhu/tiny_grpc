package config

import (
	"io/ioutil"

	log "github.com/golang/glog"
	yaml "gopkg.in/yaml.v2"
)

var rpcConf RPCConf
var clientMap map[string]*ClientService

func Init() {
	content, err := ioutil.ReadFile("tiny_grpc.yaml")
	if err != nil {
		log.Fatal("Read tiny_grpc.yaml fail, err:%+v", err.Error())
		return
	}
	err = yaml.Unmarshal(content, &rpcConf)
	if err != nil {
		log.Fatal("Failed to parse tiny_grpc.yaml, err:%+v", err)
	}

	if rpcConf.ZK == nil || rpcConf.ZK.Addr == "" {
		log.Fatal("tiny_grpc.yaml not defined zookeeper addr")
	}

	clientMap = make(map[string]*ClientService)
	for _, v := range rpcConf.Client.Service {
		clientMap[v.Name] = v
	}

	log.Info("Parse tiny_grpc.yaml suc.")
}

type ZKConf struct {
	Timeout int    `yaml:"timeout"`
	Addr    string `yaml:"addr"`
	Expire  int    `yaml:"expire"`
}

type ClientConf struct {
	Service []*ClientService `yaml:"service"`
}

type ServerConf struct {
	Service *ServerService `yaml:"service"`
}

type PoolConf struct {
	Capacity       int    `yaml:"capacity"`
	ExpireInterval int    `yaml:"expire_interval"`
	Timeout        int    `yaml:"timeout"` //连接池建立连接超时时间
	Network        string `yaml:"network"` //连接池网络类型，目前只支持tcp
}

type ClientService struct {
	Name              string         `yaml:"name"`
	NetWork           string         `yaml:"network"`
	Timeout           int            `yaml:"timeout"`
	SerializationType string         `yaml:"serialization_type"`
	UseConnPool       bool           `yaml:"use_connpool"`
	ConnPool          *PoolConf      `yaml:"connpool"`
	UseHystrix        bool           `yaml:"use_hystrix"`
	Hystrix           *HystrixConfig `yaml:"hystrix"`
}

type HystrixConfig struct {
	Timeout                int `yaml:"timeout"`
	MaxConcurrentRequests  int `yaml:"max_concurrent_requests"`
	RequestVolumeThreshold int `yaml:"request_volume_threshold"`
	SleepWindow            int `yaml:"sleep_window"`
	ErrorPercentThreshold  int `yaml:"error_percent_threshold"`
}

type ServerService struct {
	Name        string    `yaml:"name"`
	Timeout     int       `yaml:"timeout"`
	UseWorkPool bool      `yaml:"use_workpool"`
	NetWork     string    `yaml:"network"`
	WorkPool    *PoolConf `yaml:"workpool"`
}

type RPCConf struct {
	ZK     *ZKConf     `yaml:"zookeeper"`
	Client *ClientConf `yaml:"client"`
	Server *ServerConf `yaml:"server"`
}
