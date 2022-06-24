package config

func GetServerConfig() *ServerService {
	if rpcConf.Server != nil && rpcConf.Server.Service != nil {
		return rpcConf.Server.Service
	}
	// 没有手动配置的话,使用默认配置
	return &ServerService{
		Name:        "test",
		Timeout:     1000,
		UseWorkPool: true,
		NetWork:     "tcp",
		WorkPool: &PoolConf{
			Capacity:       10000,
			ExpireInterval: 5000,
			Timeout:        1000,
		},
	}
}

func GetClientConfig(path string) *ClientService {
	if v, ok := clientMap[path]; ok {
		return v
	}
	// 如果配置文件没有配置则使用默认值
	return &ClientService{
		NetWork:     "tcp",
		Timeout:     1000,
		UseConnPool: true,
		ConnPool: &PoolConf{
			Capacity:       10000,
			ExpireInterval: 5000,
			Timeout:        1000,
		},
		UseHystrix: false,
	}
}

// 获取注册中心配置
func GetRegistrationConfig() *ZKConf {
	if rpcConf.ZK.Timeout == 0 {
		rpcConf.ZK.Timeout = 2000
	}
	return rpcConf.ZK
}

// 获取注册中心地址
func GetRegistrationAddr() string {
	return rpcConf.ZK.Addr
}
