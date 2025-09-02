package pg

import (
	"ceph-core-api/internal/core/rados"
	"ceph-core-api/internal/logger"
	"encoding/json"
	"fmt"
)

// PGInfoService PG 및 Pool 정보 조회 서비스
type PGInfoService struct{}

// NewPGInfoService 새로운 PG 정보 서비스 인스턴스 생성
func NewPGInfoService() *PGInfoService {
	return &PGInfoService{}
}

// GetPGList ceph pg ls 명령어 실행
func (s *PGInfoService) GetPGList() (string, error) {
	logger.Infof("PG 목록 조회 서비스 시작")

	cmd := map[string]interface{}{
		"prefix": "pg ls",
		"format": "json-pretty",
	}

	return s.executeMonCommand(cmd, "PG 목록 조회")
}

// GetPGListByPool ceph pg ls-by-pool <pool-name> 명령어 실행
func (s *PGInfoService) GetPGListByPool(poolName string) (string, error) {
	logger.Infof("풀별 PG 목록 조회 서비스 시작: %s", poolName)

	// ceph pg ls-by-pool <pool-name> 명령
	cmd := map[string]interface{}{
		"prefix":  "pg ls-by-pool",
		"poolstr": poolName,
		"format":  "json-pretty",
	}

	return s.executeMonCommand(cmd, fmt.Sprintf("풀별 PG 목록 조회: %s", poolName))
}

// GetPGListByPoolID ceph pg ls [<pool:int>] 명령어 실행
func (s *PGInfoService) GetPGListByPoolID(poolID int) (string, error) {
	logger.Infof("풀 ID별 PG 목록 조회 서비스 시작: %d", poolID)

	// ceph pg ls <pool:int> 명령
	cmd := map[string]interface{}{
		"prefix": "pg ls",
		"pool":   poolID,
		"format": "json-pretty",
	}

	return s.executeMonCommand(cmd, fmt.Sprintf("풀 ID별 PG 목록 조회: %d", poolID))
}

// GetPoolListDetail ceph osd pool ls detail 명령어 실행
func (s *PGInfoService) GetPoolListDetail() (string, error) {
	logger.Infof("풀 상세 목록 조회 서비스 시작")

	// ceph osd pool ls detail 명령
	cmd := map[string]interface{}{
		"prefix": "osd pool ls",
		"detail": "detail",
		"format": "json-pretty",
	}

	return s.executeMonCommand(cmd, "풀 상세 목록 조회")
}

// GetOSDTree ceph osd tree 명령어 실행
func (s *PGInfoService) GetOSDTree() (string, error) {
	logger.Infof("OSD 트리 조회 서비스 시작")

	// ceph osd tree 명령
	cmd := map[string]interface{}{
		"prefix": "osd tree",
		"format": "json-pretty",
	}

	return s.executeMonCommand(cmd, "OSD 트리 조회")
}

// executeMonCommand MonCommand 실행 공통 함수
func (s *PGInfoService) executeMonCommand(cmd map[string]interface{}, operation string) (string, error) {
	conn, err := rados.GetConnection()
	if err != nil {
		logger.Errorf("Ceph 클러스터 연결 실패: %v", err)
		return "", fmt.Errorf("Ceph 클러스터 연결 실패: %v", err)
	}

	cmdJSON, err := json.Marshal(cmd)
	if err != nil {
		logger.Errorf("%s 명령 JSON 마샬링 실패: %v", operation, err)
		return "", fmt.Errorf("%s 명령 JSON 마샬링 실패: %v", operation, err)
	}

	logger.Debugf("%s 명령 실행: %s", operation, string(cmdJSON))

	// MonCommand 실행
	buf, info, err := conn.MonCommand(cmdJSON)
	if err != nil {
		logger.Errorf("%s 명령 실행 실패: %v", operation, err)
		return "", fmt.Errorf("%s 명령 실행 실패: %v", operation, err)
	}

	// info가 있으면 로깅
	if len(info) > 0 {
		logger.Debugf("%s 명령 정보: %s", operation, string(info))
	}

	result := string(buf)
	logger.Infof("%s 완료", operation)
	return result, nil
}
