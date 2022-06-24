package monitor

import (
	"net/http"

	log "github.com/golang/glog"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// 使用普罗米修思监控数据运行情况
func init() {
	log.Info("monitor start")
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		http.ListenAndServe("0.0.0.0:2112", nil)
	}()

}
