package device

import (
	"github.com/prometheus/client_golang/prometheus"
	_ "strconv"
)

var (
	deviceAvailableDesc = prometheus.NewDesc(
		"ceph_device_available",
		"Ceph 디바이스 사용 가능 여부 (1=가능, 0=불가)",
		[]string{"host", "path", "type", "device_id", "model", "size_gb"},
		nil,
	)
	deviceSizeBytesDesc = prometheus.NewDesc(
		"ceph_device_size_bytes",
		"Ceph 디바이스 용량 (bytes)",
		[]string{"host", "path", "type", "device_id", "model"},
		nil,
	)
)

type DeviceCollector struct{}

func (c *DeviceCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- deviceAvailableDesc
	ch <- deviceSizeBytesDesc
}

func (c *DeviceCollector) Collect(ch chan<- prometheus.Metric) {
	hosts, err := GetDevices()
	if err != nil {
		return
	}

	for _, host := range hosts {
		for _, d := range host.Devices {
			available := 0.0
			if d.Available {
				available = 1.0
			}

			sizeGB := d.SysAPI.HumanReadableSize
			if idx := len(sizeGB) - 3; idx > 0 && sizeGB[idx:] == " GB" {
				sizeGB = sizeGB[:idx]
			}

			ch <- prometheus.MustNewConstMetric(
				deviceAvailableDesc,
				prometheus.GaugeValue,
				available,
				host.Name, d.Path, d.HumanReadableType, d.DeviceID, d.SysAPI.Model, sizeGB,
			)

			ch <- prometheus.MustNewConstMetric(
				deviceSizeBytesDesc,
				prometheus.GaugeValue,
				float64(d.SysAPI.Size),
				host.Name, d.Path, d.HumanReadableType, d.DeviceID, d.SysAPI.Model,
			)
		}
	}
}
