package api

import (
	"ceph-core-api/internal/api/config"
	"github.com/gin-gonic/gin"
)

// Server HTTP API 서버
type Server struct {
	router *gin.Engine
	config *config.Config
}

// NewServer 새로운 API 서버 인스턴스 생성
func NewServer(cfg *config.Config) *Server {
	r := gin.Default()

	server := &Server{
		router: r,
		config: cfg,
	}

	// 라우터 설정
	server.setupRoutes()

	return server
}

// Run 서버 시작
func (s *Server) Run() error {
	return s.router.Run(s.config.ServerAddress)
}
