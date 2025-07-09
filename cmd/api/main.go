package main

import (
	"ceph-core-api/internal/api"
	"ceph-core-api/internal/config"
	"ceph-core-api/internal/core/rados"
	"ceph-core-api/internal/utils"
	"go.uber.org/zap"
)

var logger *zap.SugaredLogger

func init() {
	logger = utils.GetLogger()
}

func main() {
	// 설정 로드
	cfg := config.NewConfig()

	// 종료 시 정리
	defer rados.CloseConnection()

	// API 서버 시작
	server := api.NewServer(cfg)
	logger.Infof("서버 시작: %s", cfg.ServerAddress)
	if err := server.Run(); err != nil {
		logger.Fatalf("서버 실행 실패: %v", err)
	}
}

// Ceph 연결 생성
//cephConn, err := rados.NewCephConnection()
//if err != nil {
//logger.Fatalf("Ceph 연결 생성 실패: %v", err)
//}
//defer cephConn.Close()
//
//// 기본 설정으로 연결
//err = cephConn.ConnectWithDefaultConfig()
//if err != nil {
//logger.Fatalf("Ceph 클러스터 연결 실패: %v", err)
//}
//
//// 연결 상태 확인
//if !cephConn.IsConnected() {
//logger.Fatal("Ceph 클러스터 연결 상태 확인 실패")
//}
//
//// RBD 이미지 PG/OSD 매핑 조회 테스트
//poolName := "swimming"
//imageName := "img3"
//
//logger.Infof("RBD 이미지 PG/OSD 매핑 조회: %s/%s", poolName, imageName)
//results, err := rbd.MapUsedObjectsToOSDs(cephConn.GetConnection(), poolName, imageName, 64)
//if err != nil {
//return
//}
//
//data, err := json.Marshal(results)
//if err != nil {
//logger.Error(err)
//}
//logger.Info(string(data))
