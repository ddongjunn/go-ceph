package rados

import (
	"ceph-core-api/internal/logger"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ceph/go-ceph/rados"
)

var (
	instance *rados.Conn
	once     sync.Once
	mu       sync.Mutex
)

// 연결 타임아웃 설정 (초 단위)
const connectionTimeout = 5

var ErrConnectionTimeout = errors.New("Ceph 연결 타임아웃 오류")

func GetConnection() (*rados.Conn, error) {
	var initError error

	mu.Lock()
	if instance != nil {
		mu.Unlock()
		return instance, nil
	}
	mu.Unlock()

	// 타임아웃 처리를 위한 채널
	done := make(chan struct {
		conn *rados.Conn
		err  error
	}, 1)

	// 고루틴으로 연결 시도
	go func() {
		once.Do(func() {
			conn, err := rados.NewConn()
			if err != nil {
				logger.Errorf("Ceph 연결 객체 생성 실패: %v", err)
				initError = err
				return
			}

			err = conn.ReadDefaultConfigFile()
			if err != nil {
				logger.Errorf("Ceph 설정 파일 읽기 실패: %v", err)
				conn.Shutdown()
				initError = err
				return
			}

			err = conn.Connect()
			if err != nil {
				logger.Errorf("Ceph 클러스터 연결 실패: %v", err)
				conn.Shutdown()
				initError = err
				return
			}

			logger.Infof("Ceph 클러스터 연결 성공")
			instance = conn
		})

		done <- struct {
			conn *rados.Conn
			err  error
		}{conn: instance, err: initError}
	}()

	// 타임아웃 또는 완료 대기
	select {
	case result := <-done:
		if result.err != nil {
			return nil, result.err
		}
		if result.conn == nil {
			return nil, errors.New("Ceph 연결 초기화 실패")
		}
		return result.conn, nil
	case <-time.After(connectionTimeout * time.Second):
		logger.Warnf("Ceph 연결 시도 타임아웃")
		return nil, ErrConnectionTimeout
	}
}

// CloseConnection 연결 종료 (프로그램 종료 시)
func CloseConnection() {
	mu.Lock()
	defer mu.Unlock()

	if instance != nil {
		instance.Shutdown()
		instance = nil
	}

	// 재초기화를 위해 once 리셋 (테스트 환경 등에서 유용)
	once = sync.Once{}
}

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

	logger.Infof("Ceph 클러스터 연결 성공")
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
		logger.Infof("Ceph 연결 종료")
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
