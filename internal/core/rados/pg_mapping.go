package rados

import (
	"encoding/json"
	"fmt"

	"github.com/ceph/go-ceph/rados"
	"github.com/ceph/go-ceph/rbd"
)

// PGMapping 구조체
type PGMapping struct {
	PG   string
	OSDs []int
}

func GetClusterFSID() string {
	conn, err := rados.NewConn()
	if err != nil {
		logger.Errorf("Ceph 연결 실패: %v", err)
		return ""
	}

	err = conn.ReadConfigFile("/etc/notused/notused.conf")
	if err != nil {
		logger.Errorf("Ceph 설정 파일 읽기 실패: %v", err)
		return ""
	}

	err = conn.Connect()
	if err != nil {
		logger.Errorf("Ceph 클러스터 연결 실패: %v", err)
		return ""
	}
	defer conn.Shutdown()

	// FSID 출력
	fsid, err := conn.GetFSID()
	if err != nil {
		logger.Errorf("FSID 가져오기 실패: %v", err)
		return ""
	}
	logger.Infof("fsid: %s", fsid)

	// MgrCommand
	command, err := json.Marshal(
		map[string]string{"prefix": "get_command_descriptions", "format": "json"})
	buf, _, err := conn.MgrCommand([][]byte{command})

	var message map[string]interface{}
	err = json.Unmarshal(buf, &message)

	return fsid
}

func PrintPgDump(conn *rados.Conn) {
	command, err := json.Marshal(
		map[string]string{"prefix": "pg dump", "format": "json"})
	if err != nil {
		logger.Errorf("JSON 마샬링 실패: %v", err)
		return
	}

	buf, _, err := conn.MonCommand(command)
	if err != nil {
		logger.Errorf("MonCommand 실패: %v", err)
		return
	}

	var pgDump map[string]interface{}
	err = json.Unmarshal(buf, &pgDump)
	if err != nil {
		logger.Errorf("JSON 언마샬링 실패: %v", err)
		return
	}

	logger.Infof("PG Dump: %+v", pgDump)
}

func ListPoolsAndImages(conn *rados.Conn) (map[string][]string, error) {
	pools, err := GetPools(conn)
	if err != nil {
		return nil, fmt.Errorf("풀 목록 가져오기 실패: %v", err)
	}

	result := make(map[string][]string)
	for _, pool := range pools {
		images, err := ListRBDImages(conn, pool)
		if err != nil {
			logger.Warnf("풀 %s의 이미지 목록 가져오기 실패: %v", pool, err)
			continue
		}
		result[pool] = images
	}

	return result, nil
}

func GetPoolIdByName(conn *rados.Conn, poolName string) int64 {
	poolId, err := conn.GetPoolByName(poolName)
	if err != nil {
		logger.Errorf("풀 ID 가져오기 실패: %v", err)
		return -1
	}
	return poolId
}

func GetPools(conn *rados.Conn) ([]string, error) {
	pools, err := conn.ListPools()
	if err != nil {
		return nil, fmt.Errorf("풀 목록 가져오기 실패: %v", err)
	}
	return pools, nil
}

func GetImages(conn *rados.Conn, poolName string) ([]map[string]interface{}, error) {
	ioctx, err := conn.OpenIOContext(poolName)
	if err != nil {
		return nil, fmt.Errorf("IOContext 열기 실패: %v", err)
	}
	defer ioctx.Destroy()

	imageNames, err := rbd.GetImageNames(ioctx)
	if err != nil {
		return nil, fmt.Errorf("이미지 이름 목록 가져오기 실패: %v", err)
	}

	var images []map[string]interface{}
	for _, imageName := range imageNames {
		image := rbd.GetImage(ioctx, imageName)
		err = image.Open()
		if err != nil {
			logger.Warnf("이미지 %s 열기 실패: %v", imageName, err)
			continue
		}

		stat, err := image.Stat()
		if err != nil {
			logger.Warnf("이미지 %s 정보 가져오기 실패: %v", imageName, err)
			image.Close()
			continue
		}

		images = append(images, map[string]interface{}{
			"name": imageName,
			"size": stat.Size,
		})

		image.Close()
	}

	return images, nil
}

// ListRBDImages 풀의 RBD 이미지 목록을 문자열 배열로 반환합니다.
func ListRBDImages(conn *rados.Conn, poolName string) ([]string, error) {
	ioctx, err := conn.OpenIOContext(poolName)
	if err != nil {
		return nil, fmt.Errorf("IOContext 열기 실패: %v", err)
	}
	defer ioctx.Destroy()

	imageNames, err := rbd.GetImageNames(ioctx)
	if err != nil {
		return nil, fmt.Errorf("이미지 이름 목록 가져오기 실패: %v", err)
	}

	return imageNames, nil
}
