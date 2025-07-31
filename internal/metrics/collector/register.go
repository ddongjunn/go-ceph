package collector

import (
	"ceph-core-api/internal/metrics/collector/device"
	"github.com/prometheus/client_golang/prometheus"
)

func RegisterCollectors() {
	prometheus.MustRegister(&device.DeviceCollector{})
}
