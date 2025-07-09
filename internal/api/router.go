package api

import (
	"ceph-core-api/internal/api/handlers"
)

// setupRoutes API 경로 설정
func (s *Server) setupRoutes() {
	// API 그룹
	api := s.router.Group("/api")
	{
		// 클러스터 정보
		api.GET("/cluster/fsid", handlers.GetClusterFSID)
	}
}
