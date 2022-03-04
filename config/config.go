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

func GetWPoolCap() int32 {
	return 10000
}

func GetWPoolExpiredInterval() int64 {
	return 10
}
