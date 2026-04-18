package cluster

import (
	"ceph-core-api/internal/core/rados"
	"ceph-core-api/internal/logger"
)

// InfoService 클러스터 정보 조회 서비스
type InfoService struct{}

// NewInfoService 새로운 클러스터 정보 서비스 인스턴스 생성
func NewInfoService() *InfoService {
	return &InfoService{}
}

// GetFSID Ceph 클러스터 FSID 조회 (연결 관리 포함)
func (s *InfoService) GetFSID() (string, error) {
	logger.Infof("클러스터 FSID 조회 서비스 시작")

	// Ceph 클러스터 연결
	conn, err := rados.GetConnection()
	if err != nil {
		logger.Errorf("Ceph 클러스터 연결 실패: %v", err)
		return "", err
	}
	defer conn.Shutdown()

	// FSID 조회
	fsid, err := conn.GetFSID()
	if err != nil {
		logger.Errorf("클러스터 FSID 조회 실패: %v", err)
		return "", err
	}

	logger.Infof("클러스터 FSID 조회 완료: %s", fsid)
	return fsid, nil
}
