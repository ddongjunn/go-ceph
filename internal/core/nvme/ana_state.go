package nvme

import (
	"ceph-core-api/internal/core/ssh"
	"ceph-core-api/internal/logger"
	"fmt"
	"strings"
)

// ANAStateService NVMe ANA 상태 설정 서비스
type ANAStateService struct{}

// NewANAStateService 새로운 ANA 상태 서비스 인스턴스 생성
func NewANAStateService() *ANAStateService {
	return &ANAStateService{}
}

// ANAStateConfig ANA 상태 설정 구조체
type ANAStateConfig struct {
	ContainerName string `json:"container_name" binding:"required"`
	ANAState      string `json:"ana_state" binding:"required"`      // optimized, non_optimized, inaccessible
	Transport     string `json:"transport" binding:"required"`      // tcp, rdma
	Address       string `json:"address" binding:"required"`        // IP 주소
	Port          int    `json:"port" binding:"required"`           // 포트 번호
	NQN           string `json:"nqn" binding:"required"`            // NVMe Qualified Name
}

// findRPCScript 컨테이너 내부에서 rpc.py 스크립트 경로 찾기
func (s *ANAStateService) findRPCScript(host, user, authMethod, authValue, containerName string) (string, error) {
	// find 명령으로 rpc.py 스크립트 검색
	findCommand := fmt.Sprintf("podman exec %s find / -name 'rpc.py' -type f 2>/dev/null | head -1", containerName)
	result, err := ssh.ExecuteSSHCommand(host, user, authMethod, authValue, findCommand, 30)
	if err != nil {
		return "", fmt.Errorf("rpc.py 스크립트를 찾을 수 없습니다: %v", err)
	}

	foundPath := strings.TrimSpace(result)
	if foundPath == "" {
		return "", fmt.Errorf("컨테이너 내부에 rpc.py 스크립트가 존재하지 않습니다")
	}

	logger.Infof("rpc.py 스크립트 발견: %s", foundPath)
	return foundPath, nil
}

// SetANAState NVMe 서브시스템 리스너의 ANA 상태를 설정
func (s *ANAStateService) SetANAState(host, user, authMethod, authValue string, config *ANAStateConfig) (string, error) {
	logger.Infof("NVMe ANA 상태 설정: %s -> %s", config.ContainerName, config.ANAState)

	// rpc.py 스크립트 경로 찾기
	rpcScriptPath, err := s.findRPCScript(host, user, authMethod, authValue, config.ContainerName)
	if err != nil {
		return "", err
	}

	// podman exec 명령 구성
	command := fmt.Sprintf(
		"podman exec %s %s nvmf_subsystem_listener_set_ana_state -n %s -t %s -a %s -s %d %s",
		config.ContainerName,
		rpcScriptPath,
		config.ANAState,
		config.Transport,
		config.Address,
		config.Port,
		config.NQN,
	)

	logger.Debugf("실행 명령: %s", command)

	// SSH를 통해 명령 실행
	result, err := ssh.ExecuteSSHCommand(host, user, authMethod, authValue, command, 30)
	if err != nil {
		logger.Errorf("ANA 상태 설정 실패: %v", err)
		return "", fmt.Errorf("ANA 상태 설정 실패: %v", err)
	}

	logger.Infof("ANA 상태 설정 완료: %s", config.ANAState)
	return strings.TrimSpace(result), nil
}
