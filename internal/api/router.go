package api

import (
	"ceph-core-api/internal/api/handlers"
	"ceph-core-api/internal/metrics/collector"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func (s *Server) setupRoutes() {
	api := s.router.Group("/api")
	{
		api.GET("/cluster/fsid", handlers.GetClusterFSID)
	}

	v1 := s.router.Group("/api/v1")
	{
		v1.GET("/pgs", handlers.GetPGList) // ceph pg ls

		v1.GET("/pools", handlers.GetPoolListDetail)                  // ceph osd pool ls detail
		v1.GET("/pool/name/:pool_name/pgs", handlers.GetPGListByPool) // ceph pg ls-by-pool <pool-name>
		v1.GET("/pool/id/:pool_id/pgs", handlers.GetPGListByPoolID)   // ceph pg ls [<pool:int>]

		v1.GET("/osd/tree", handlers.GetOSDTree) // ceph osd tree

		v1.PUT("/nvme/ana-state", handlers.SetNVMeANAStateHandler)  // NVMe ANA 상태 설정
		v1.GET("/nvme/subsystems", handlers.GetNVMeANAStateHandler) // NVMe 서브시스템 상태 조회
	}

	collector.RegisterCollectors()
	s.router.GET("/metrics", gin.WrapH(promhttp.Handler()))
}
