package rados

import (
	"fmt"

	"github.com/ceph/go-ceph/rados"
)

type CephConnection struct {
	conn *rados.Conn
}

// NewCephConnection 새로운 Ceph 연결을 생성
func NewCephConnection() (*CephConnection, error) {
	conn, err := rados.NewConn()
	if err != nil {
		return nil, fmt.Errorf("Ceph 연결 생성 실패: %v", err)
	}
	return &CephConnection{conn: conn}, nil
}

// ConnectWithDefaultConfig 기본 설정 파일로 Ceph 클러스터에 연결
func (c *CephConnection) ConnectWithDefaultConfig() error {
	err := c.conn.ReadDefaultConfigFile()
	if err != nil {
		return fmt.Errorf("Ceph 설정 파일 읽기 실패: %v", err)
	}

	err = c.conn.Connect()
	if err != nil {
		return fmt.Errorf("Ceph 클러스터 연결 실패: %v", err)
	}

	logger.Info("Ceph 클러스터 연결 성공")
	return nil
}

// ConnectWithConfigFile 지정된 설정 파일로 Ceph 클러스터에 연결
func (c *CephConnection) ConnectWithConfigFile(configPath string) error {
	err := c.conn.ReadConfigFile(configPath)
	if err != nil {
		return fmt.Errorf("Ceph 설정 파일 읽기 실패 (%s): %v", configPath, err)
	}

	err = c.conn.Connect()
	if err != nil {
		return fmt.Errorf("Ceph 클러스터 연결 실패: %v", err)
	}

	logger.Infof("Ceph 클러스터 연결 성공 (설정: %s)", configPath)
	return nil
}

// GetConnection 내부 rados.Conn 객체를 반환
func (c *CephConnection) GetConnection() *rados.Conn {
	return c.conn
}

// Close Ceph 연결을 종료
func (c *CephConnection) Close() {
	if c.conn != nil {
		c.conn.Shutdown()
		logger.Info("Ceph 연결 종료")
	}
}

// GetClusterStats 클러스터 통계 정보를 조회
func (c *CephConnection) GetClusterStats() (*rados.ClusterStat, error) {
	stat, err := c.conn.GetClusterStats()
	if err != nil {
		return nil, fmt.Errorf("클러스터 통계 조회 실패: %v", err)
	}
	return &stat, nil
}

// IsConnected 연결 상태를 확인
func (c *CephConnection) IsConnected() bool {
	if c.conn == nil {
		return false
	}

	// 간단한 연결 테스트 (FSID 조회)
	_, err := c.conn.GetFSID()
	return err == nil
}
