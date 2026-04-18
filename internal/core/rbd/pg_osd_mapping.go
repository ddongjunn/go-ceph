package rbd

import (
	"ceph-core-api/internal/logger"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ceph/go-ceph/rados"
	"github.com/ceph/go-ceph/rbd"
)

type PGOSDEntry struct {
	PGID string `json:"pg"`
	OSDs []int  `json:"osds"`
}

func MapUsedObjectsToOSDs(
	conn *rados.Conn,
	poolName, imageName string,
	workerCount int,
) ([]PGOSDEntry, error) {
	startTime := time.Now()

	ioctx, err := conn.OpenIOContext(poolName)
	if err != nil {
		return nil, fmt.Errorf("IOContext 열기 실패: %v", err)
	}
	defer ioctx.Destroy()

	img := rbd.GetImage(ioctx, imageName)
	if err := img.Open(rbd.NoSnapshot); err != nil {
		return nil, fmt.Errorf("RBD 이미지 열기 실패: %v", err)
	}
	defer img.Close()

	stat, err := img.Stat()
	if err != nil {
		return nil, fmt.Errorf("이미지 정보 가져오기 실패: %v", err)
	}
	logger.Infof("이미지 크기: %.2f MB (%d bytes)", float64(stat.Size)/1024/1024, stat.Size)

	objectPrefix := stat.Block_name_prefix
	objectSize := uint64(1) << stat.Order

	var objectNames []string
	cfg := rbd.DiffIterateConfig{
		Offset:      0,
		Length:      stat.Size,
		WholeObject: rbd.DiffWholeObject(1), // 전체 객체 단위로 리턴
		Callback: func(offset, length uint64, exists int, data interface{}) int {
			if exists == 1 {
				objIndex := offset / objectSize
				objName := fmt.Sprintf("%s.%012x", objectPrefix, objIndex)
				objectNames = append(objectNames, objName)
			}
			return 0
		},
	}
	if err := img.DiffIterate(cfg); err != nil {
		return nil, fmt.Errorf("DiffIterate 실패: %v", err)
	}

	if len(objectNames) == 0 {
		logger.Warnf("사용 중인 객체가 없습니다.")
		return nil, nil
	}

	// 2. 고유 PGID 수집
	pgSet := make(map[string]struct{})
	var pgSetMu sync.Mutex
	objChan := make(chan string, len(objectNames))
	for _, name := range objectNames {
		objChan <- name
	}
	close(objChan)

	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for obj := range objChan {
				pgid, err := getPGIDForObject(conn, poolName, obj)
				if err != nil {
					logger.Warnf("PG 조회 실패: %s → %v", obj, err)
					continue
				}
				pgSetMu.Lock()
				pgSet[pgid] = struct{}{}
				pgSetMu.Unlock()
			}
		}()
	}
	wg.Wait()

	// 3. 고유 PGID → OSD 병렬 매핑
	pgList := make([]string, 0, len(pgSet))
	for pg := range pgSet {
		pgList = append(pgList, pg)
	}
	sort.Strings(pgList)

	pgChan := make(chan string, len(pgList))
	resChan := make(chan PGOSDEntry, len(pgList))
	errCount := 0

	for _, pg := range pgList {
		pgChan <- pg
	}
	close(pgChan)

	var wg2 sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg2.Add(1)
		go func() {
			defer wg2.Done()
			for pg := range pgChan {
				osds, err := getOSDsForPGID(conn, pg)
				if err != nil {
					pgSetMu.Lock()
					errCount++
					if errCount <= 5 {
						logger.Warnf("PG 매핑 실패: %s → %v", pg, err)
					}
					pgSetMu.Unlock()
					continue
				}
				resChan <- PGOSDEntry{PGID: pg, OSDs: osds}
			}
		}()
	}
	wg2.Wait()
	close(resChan)

	var results []PGOSDEntry
	for entry := range resChan {
		results = append(results, entry)
	}

	logger.Infof("OSD 매핑 완료: %d 성공, %d 실패", len(results), errCount)
	logger.Infof("총 분석 시간: %v", time.Since(startTime))
	return results, nil
}

func MapPGsToOSDs(
	conn *rados.Conn,
	poolName, imageName string,
	workerCount int,
) ([]PGOSDEntry, error) {
	startTime := time.Now()

	// Step 1. Open IOContext and image
	ioctx, err := conn.OpenIOContext(poolName)
	if err != nil {
		return nil, fmt.Errorf("IOContext 열기 실패: %v", err)
	}
	defer ioctx.Destroy()

	img := rbd.GetImage(ioctx, imageName)
	if err := img.Open(); err != nil {
		return nil, fmt.Errorf("RBD 이미지 열기 실패: %v", err)
	}
	defer img.Close()

	stat, err := img.Stat()
	if err != nil {
		return nil, fmt.Errorf("이미지 정보 가져오기 실패: %v", err)
	}
	logger.Infof("이미지 크기: %.2f MB (%d bytes)", float64(stat.Size)/1024/1024, stat.Size)

	// Step 2. 객체 이름 수집
	objectPrefix := stat.Block_name_prefix
	iter, err := ioctx.Iter()
	if err != nil {
		return nil, fmt.Errorf("객체 iterator 생성 실패: %v", err)
	}
	defer iter.Close()

	var objectNames []string
	for iter.Next() {
		name := iter.Value()
		if strings.HasPrefix(name, objectPrefix) {
			objectNames = append(objectNames, name)
		}
	}
	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("객체 순회 오류: %v", err)
	}
	if len(objectNames) == 0 {
		logger.Warnf("분석할 객체가 없습니다.")
		return nil, nil
	}

	// Step 3. 병렬로 고유 PGID 수집
	pgSet := make(map[string]struct{})
	var pgSetMu sync.Mutex
	objectChan := make(chan string, len(objectNames))

	for _, name := range objectNames {
		objectChan <- name
	}
	close(objectChan)

	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for obj := range objectChan {
				pg, err := getPGIDForObject(conn, poolName, obj)
				if err != nil {
					logger.Warnf("PG 조회 실패: %s → %v", obj, err)
					continue
				}
				pgSetMu.Lock()
				pgSet[pg] = struct{}{}
				pgSetMu.Unlock()
			}
		}()
	}
	wg.Wait()

	if len(pgSet) == 0 {
		logger.Warnf("PG 정보가 수집되지 않았습니다.")
		return nil, nil
	}

	// Step 4. PG별 OSD 병렬 조회
	pgList := make([]string, 0, len(pgSet))
	for pg := range pgSet {
		pgList = append(pgList, pg)
	}
	sort.Strings(pgList)

	pgChan := make(chan string, len(pgList))
	resultChan := make(chan PGOSDEntry, len(pgList))

	for _, pg := range pgList {
		pgChan <- pg
	}
	close(pgChan)

	errCount := 0
	var resultWg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		resultWg.Add(1)
		go func() {
			defer resultWg.Done()
			for pg := range pgChan {
				osds, err := getOSDsForPGID(conn, pg)
				if err != nil {
					pgSetMu.Lock()
					errCount++
					if errCount <= 5 {
						logger.Warnf("PG 매핑 실패: %s → %v", pg, err)
					}
					pgSetMu.Unlock()
					continue
				}
				resultChan <- PGOSDEntry{PGID: pg, OSDs: osds}
			}
		}()
	}
	resultWg.Wait()
	close(resultChan)

	var entries []PGOSDEntry
	for res := range resultChan {
		entries = append(entries, res)
	}

	logger.Infof("OSD 매핑 성공/실패: %d / %d", len(entries), errCount)
	logger.Infof("총 분석 시간: %v", time.Since(startTime))

	return entries, nil
}

func getPGIDForObject(conn *rados.Conn, poolName, objectName string) (string, error) {
	cmd := map[string]interface{}{
		"prefix": "osd map",
		"pool":   poolName,
		"object": objectName,
		"format": "json",
	}
	buf, _, err := conn.MonCommand(encodeJSON(cmd))
	if err != nil {
		return "", fmt.Errorf("MonCommand 실패: %v", err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(buf, &m); err != nil {
		return "", fmt.Errorf("JSON 언마샬링 실패: %v", err)
	}
	if pgid, ok := m["pgid"].(string); ok {
		return pgid, nil
	}
	return "", fmt.Errorf("pgid 필드 없음")
}

func getOSDsForPGID(conn *rados.Conn, pgid string) ([]int, error) {
	cmd := map[string]interface{}{
		"prefix": "pg map",
		"pgid":   pgid,
		"format": "json",
	}
	buf, _, err := conn.MonCommand(encodeJSON(cmd))
	if err != nil {
		return nil, err
	}
	var m map[string]interface{}
	if err := json.Unmarshal(buf, &m); err != nil {
		return nil, err
	}
	up, ok := m["up"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("up 필드 없음")
	}
	osds := make([]int, 0, len(up))
	for _, v := range up {
		if f, ok := v.(float64); ok {
			osds = append(osds, int(f))
		}
	}
	return osds, nil
}

func encodeJSON(v interface{}) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("JSON 직렬화 실패: %v", err))
	}
	return b
}
