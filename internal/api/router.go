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

	collector.RegisterCollectors()
	s.router.GET("/metrics", gin.WrapH(promhttp.Handler()))
}
