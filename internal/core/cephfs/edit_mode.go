package cephfs

import (
	"ceph-core-api/internal/core/rados"
	"ceph-core-api/internal/logger"
	"encoding/json"
	"fmt"

	cephrados "github.com/ceph/go-ceph/rados"
)

type EditService struct{}

func NewEditService() *EditService {
	return &EditService{}
}

// DisableForEdit CephFS를 편집 가능 상태로 비활성화 (연결 관리 포함)
func (s *EditService) DisableForEdit(fsName string) error {
	logger.Infof("CephFS 편집 모드 서비스 시작: %s", fsName)

	// Ceph 클러스터 연결
	conn, err := rados.GetConnection()
	if err != nil {
		logger.Errorf("Ceph 클러스터 연결 실패: %v", err)
		return err
	}

	// 실제 비즈니스 로직 실행
	err = disableCephFSForEdit(conn, fsName)
	if err != nil {
		logger.Errorf("CephFS 편집 모드 활성화 실패: %v", err)
		return err
	}

	logger.Infof("CephFS 편집 모드 서비스 완료: %s", fsName)
	return nil
}

// EnableAfterEdit CephFS를 편집 후 다시 활성화 (연결 관리 포함)
func (s *EditService) EnableAfterEdit(fsName string) error {
	logger.Infof("CephFS 편집 후 활성화 서비스 시작: %s", fsName)

	// Ceph 클러스터 연결
	conn, err := rados.GetConnection()
	if err != nil {
		logger.Errorf("Ceph 클러스터 연결 실패: %v", err)
		return err
	}

	// 실제 비즈니스 로직 실행
	err = enableCephFSAfterEdit(conn, fsName)
	if err != nil {
		logger.Errorf("CephFS 편집 후 활성화 실패: %v", err)
		return err
	}

	logger.Infof("CephFS 편집 후 활성화 서비스 완료: %s", fsName)
	return nil
}

// disableCephFSForEdit CephFS를 편집 가능 상태로 비활성화 (내부 함수)
// 1. ceph fs fail <fs_name>
// 2. ceph fs set <fs_name> refuse_client_session true
func disableCephFSForEdit(conn *cephrados.Conn, fsName string) error {
	logger.Infof("CephFS 편집 모드 활성화 시작: %s", fsName)

	// 1. fs fail 명령 실행
	failCmd := map[string]interface{}{
		"prefix":  "fs fail",
		"fs_name": fsName,
		"format":  "json",
	}
	failCmdJSON, err := json.Marshal(failCmd)
	if err != nil {
		return fmt.Errorf("fs fail 명령 JSON 마샬링 실패: %v", err)
	}

	logger.Debugf("fs fail 명령 실행: %s", string(failCmdJSON))
	_, _, err = conn.MonCommand(failCmdJSON)
	if err != nil {
		return fmt.Errorf("fs fail 명령 실행 실패: %v", err)
	}

	// 2. fs set refuse_client_session true 명령 실행
	setCmd := map[string]interface{}{
		"prefix":  "fs set",
		"fs_name": fsName,
		"var":     "refuse_client_session",
		"val":     "true",
		"format":  "json",
	}
	setCmdJSON, err := json.Marshal(setCmd)
	if err != nil {
		return fmt.Errorf("fs set refuse_client_session 명령 JSON 마샬링 실패: %v", err)
	}

	logger.Debugf("fs set refuse_client_session 명령 실행: %s", string(setCmdJSON))
	_, _, err = conn.MonCommand(setCmdJSON)
	if err != nil {
		return fmt.Errorf("fs set refuse_client_session 명령 실행 실패: %v", err)
	}

	logger.Infof("CephFS 편집 모드 활성화 완료: %s", fsName)
	return nil
}

// enableCephFSAfterEdit CephFS를 편집 후 다시 활성화 (내부 함수)
// 1. ceph fs set <fs_name> refuse_client_session false
// 2. ceph fs set <fs_name> joinable true
func enableCephFSAfterEdit(conn *cephrados.Conn, fsName string) error {
	logger.Infof("CephFS 편집 후 활성화 시작: %s", fsName)

	// 1. fs set refuse_client_session false 명령 실행
	setRefuseCmd := map[string]interface{}{
		"prefix":  "fs set",
		"fs_name": fsName,
		"var":     "refuse_client_session",
		"val":     "false",
		"format":  "json",
	}
	setRefuseCmdJSON, err := json.Marshal(setRefuseCmd)
	if err != nil {
		return fmt.Errorf("fs set refuse_client_session false 명령 JSON 마샬링 실패: %v", err)
	}

	logger.Debugf("fs set refuse_client_session false 명령 실행: %s", string(setRefuseCmdJSON))
	_, _, err = conn.MonCommand(setRefuseCmdJSON)
	if err != nil {
		return fmt.Errorf("fs set refuse_client_session false 명령 실행 실패: %v", err)
	}

	// 2. fs set joinable true 명령 실행
	setJoinableCmd := map[string]interface{}{
		"prefix":  "fs set",
		"fs_name": fsName,
		"var":     "joinable",
		"val":     "true",
		"format":  "json",
	}
	setJoinableCmdJSON, err := json.Marshal(setJoinableCmd)
	if err != nil {
		return fmt.Errorf("fs set joinable true 명령 JSON 마샬링 실패: %v", err)
	}

	logger.Debugf("fs set joinable true 명령 실행: %s", string(setJoinableCmdJSON))
	_, _, err = conn.MonCommand(setJoinableCmdJSON)
	if err != nil {
		return fmt.Errorf("fs set joinable true 명령 실행 실패: %v", err)
	}

	logger.Infof("CephFS 편집 후 활성화 완료: %s", fsName)
	return nil
}
