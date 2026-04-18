package main

import (
	"ceph-core-api/internal/api"
	"ceph-core-api/internal/api/config"
	"ceph-core-api/internal/core/rados"
	"ceph-core-api/internal/logger"
)

func main() {
	// 설정 로드
	cfg := config.NewConfig()

	// 종료 시 정리
	defer rados.CloseConnection()

	// 클러스터 연결 확인
	_, err := rados.GetConnection()
	if err != nil {
		logger.Errorf("클러스터 연결 실패: %v", err)
	}

	// API 서버 시작
	server := api.NewServer(cfg)
	logger.Infof("서버 시작: %s", cfg.ServerAddress)
	if err := server.Run(); err != nil {
		logger.Errorf("서버 실행 실패: %v", err)
	}
}
