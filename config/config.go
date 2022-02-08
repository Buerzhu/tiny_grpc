package config

func GetRegistrationAddr() []string {
	return []string{"0.0.0.0:21810"}
}

func GetBusinessName() string {
	return "/test"
}

func GetServiceName() string {
	return "/demo"
}

func GetAddrCacheExpiredTime() int64 {
	return 2
}
