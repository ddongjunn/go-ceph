package notused

import (
	"fmt"
	"sync"
	"time"

	"github.com/ceph/go-ceph/rados"
	"github.com/ceph/go-ceph/rbd"
	"go.uber.org/zap/zapcore"
)

// 🚀 최적화된 RBD 이미지 매핑 분석기
// 성능: ~1.2초 (8워커 병렬 처리, 캐시 없음)
func AnalyzeRBDImageOptimized(conn *rados.Conn, poolName, imageName string) error {
	return AnalyzeRBDImageWithWorkers(conn, poolName, imageName, 8)
}

// 🚀 워커 수를 지정할 수 있는 RBD 이미지 매핑 분석
func AnalyzeRBDImageWithWorkers(conn *rados.Conn, poolName, imageName string, workerCount int) error {
	slog.Infof("\n=== 🚀 최적화된 RBD 매핑 분석 === [워커: %d개]", workerCount)

	startTime := time.Now()

	// 1. 이미지 메타데이터 조회
	ioctx, err := conn.OpenIOContext(poolName)
	if err != nil {
		return fmt.Errorf("IOContext 열기 실패: %v", err)
	}
	defer ioctx.Destroy()

	image := rbd.GetImage(ioctx, imageName)
	err = image.Open()
	if err != nil {
		return fmt.Errorf("이미지 열기 실패: %v", err)
	}
	defer image.Close()

	stat, err := image.Stat()
	if err != nil {
		return fmt.Errorf("이미지 정보 가져오기 실패: %v", err)
	}

	slog.Info("=== 이미지 기본 정보 ===")
	slog.Infof("풀: %s, 이미지: %s", poolName, imageName)
	slog.Infof("크기: %.2f MB (%d bytes)", float64(stat.Size)/1024/1024, stat.Size)
	slog.Infof("객체 크기: %.2f MB (%d bytes)", float64(stat.Obj_size)/1024/1024, stat.Obj_size)
	slog.Infof("총 객체 수: %d개", stat.Num_objs)
	slog.Infof("워커 수: %d개", workerCount)

	// 2. 실제 존재하는 객체만 조회 (최적화 핵심)
	objectDiscoveryStart := time.Now()

	objectPrefix := stat.Block_name_prefix
	iter, err := ioctx.Iter()
	if err != nil {
		return fmt.Errorf("객체 iterator 생성 실패: %v", err)
	}
	defer iter.Close()

	var existingObjects []string
	for iter.Next() {
		objectName := iter.Value()
		// RBD 이미지의 객체인지 확인 (prefix 매칭)
		if len(objectName) > len(objectPrefix) &&
			objectName[:len(objectPrefix)] == objectPrefix {
			existingObjects = append(existingObjects, objectName)
		}
	}

	if err := iter.Err(); err != nil {
		return fmt.Errorf("객체 순회 중 오류: %v", err)
	}

	objectDiscoveryTime := time.Since(objectDiscoveryStart)
	usagePercent := float64(len(existingObjects)) / float64(stat.Num_objs) * 100

	slog.Infof("객체 발견 완료: %d개 (%.2f%% 사용률, 소요시간: %v)",
		len(existingObjects), usagePercent, objectDiscoveryTime)

	if len(existingObjects) == 0 {
		slog.Info("분석할 객체가 없습니다.")
		return nil
	}

	// 3. 병렬 처리를 위한 워커 풀 설정
	type MappingResult struct {
		ObjectName string
		PG         string
		OSDs       []int
		Error      error
	}

	// 채널 설정
	objectChan := make(chan string, len(existingObjects))
	resultChan := make(chan MappingResult, len(existingObjects))

	// 4. 워커 고루틴 시작
	var wg sync.WaitGroup
	mappingStart := time.Now()

	slog.Infof("병렬 매핑 분석 시작... (%d개 워커)", workerCount)

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for objectName := range objectChan {
				result := MappingResult{ObjectName: objectName}

				// PG 매핑 조회
				pgID, err := getObjectPGMapping(conn, poolName, objectName)
				if err != nil {
					result.Error = fmt.Errorf("PG 매핑 실패: %v", err)
					resultChan <- result
					continue
				}
				result.PG = pgID

				// OSD 매핑 조회 (캐시 없음 - 단순하고 빠름)
				osds, err := getPGOSDMapping(conn, pgID)
				if err != nil {
					result.Error = fmt.Errorf("OSD 매핑 실패: %v", err)
					resultChan <- result
					continue
				}
				result.OSDs = osds

				resultChan <- result
			}
		}(i)
	}

	// 5. 객체들을 워커에게 분배
	go func() {
		for _, objectName := range existingObjects {
			objectChan <- objectName
		}
		close(objectChan)
	}()

	// 6. 결과 수집
	pgToOSDs := make(map[string][]int)
	allOSDs := make(map[int]bool)
	pgToObjectCount := make(map[string]int)
	processedCount := 0
	errorCount := 0

	// 결과 수집 고루틴
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	for result := range resultChan {
		processedCount++

		if result.Error != nil {
			errorCount++
			if errorCount <= 3 { // 처음 3개 에러만 로깅
				slog.Warnf("매핑 실패 - 객체: %s, 오류: %v", result.ObjectName, result.Error)
			}
			continue
		}

		// 결과 수집
		pgToOSDs[result.PG] = result.OSDs
		pgToObjectCount[result.PG]++
		for _, osd := range result.OSDs {
			allOSDs[osd] = true
		}
	}

	mappingTime := time.Since(mappingStart)
	totalTime := time.Since(startTime)

	// 7. 결과 요약 출력
	slog.Info("\n=== 📊 분석 결과 요약 ===")
	slog.Infof("총 처리 시간: %v", totalTime)
	slog.Infof("객체 발견: %v, 매핑 분석: %v", objectDiscoveryTime, mappingTime)
	slog.Infof("처리된 객체: %d개 (성공: %d, 실패: %d)",
		processedCount, processedCount-errorCount, errorCount)
	slog.Infof("발견된 PG 수: %d개", len(pgToOSDs))
	slog.Infof("사용된 OSD 수: %d개", len(allOSDs))

	// 8. PG별 분포 출력 (상위 10개)
	if slog.Level() <= zapcore.InfoLevel {
		slog.Info("\n=== 🎯 PG별 객체 분포 (상위 10개) ===")

		type PGInfo struct {
			PG    string
			OSDs  []int
			Count int
		}

		var pgInfos []PGInfo
		for pg, osds := range pgToOSDs {
			pgInfos = append(pgInfos, PGInfo{
				PG:    pg,
				OSDs:  osds,
				Count: pgToObjectCount[pg],
			})
		}

		// 객체 수 기준 정렬
		for i := 0; i < len(pgInfos)-1; i++ {
			for j := i + 1; j < len(pgInfos); j++ {
				if pgInfos[i].Count < pgInfos[j].Count {
					pgInfos[i], pgInfos[j] = pgInfos[j], pgInfos[i]
				}
			}
		}

		// 상위 10개 출력
		maxShow := 10
		if len(pgInfos) < maxShow {
			maxShow = len(pgInfos)
		}

		for i := 0; i < maxShow; i++ {
			pg := pgInfos[i]
			slog.Infof("PG %s: %d개 객체 → OSDs %v",
				pg.PG, pg.Count, pg.OSDs)
		}

		if len(pgInfos) > 10 {
			slog.Infof("... 외 %d개 PG", len(pgInfos)-10)
		}
	}

	// 9. 최종 성능 요약
	objectsPerSecond := float64(len(existingObjects)) / totalTime.Seconds()
	slog.Info("\n=== ⚡ 성능 요약 ===")
	slog.Infof("처리 속도: %.1f 객체/초", objectsPerSecond)
	slog.Infof("워커당 평균: %.1f 객체/초", objectsPerSecond/float64(workerCount))

	if usagePercent < 5.0 {
		slog.Info("💡 매우 sparse한 이미지 - 효율적 분석 방식이 최적입니다")
	} else if usagePercent > 50.0 {
		slog.Info("💡 dense한 이미지 - 병렬 처리가 매우 효과적입니다")
	}

	return nil
}

// 🎯 간단한 분석 (결과만 반환, 로깅 최소화)
func GetRBDImageMapping(conn *rados.Conn, poolName, imageName string) (map[string][]int, error) {
	// 이미지 메타데이터 조회
	ioctx, err := conn.OpenIOContext(poolName)
	if err != nil {
		return nil, fmt.Errorf("IOContext 열기 실패: %v", err)
	}
	defer ioctx.Destroy()

	image := rbd.GetImage(ioctx, imageName)
	err = image.Open()
	if err != nil {
		return nil, fmt.Errorf("이미지 열기 실패: %v", err)
	}
	defer image.Close()

	stat, err := image.Stat()
	if err != nil {
		return nil, fmt.Errorf("이미지 정보 가져오기 실패: %v", err)
	}

	// 실제 존재하는 객체만 조회
	objectPrefix := stat.Block_name_prefix
	iter, err := ioctx.Iter()
	if err != nil {
		return nil, fmt.Errorf("객체 iterator 생성 실패: %v", err)
	}
	defer iter.Close()

	var existingObjects []string
	for iter.Next() {
		objectName := iter.Value()
		if len(objectName) > len(objectPrefix) &&
			objectName[:len(objectPrefix)] == objectPrefix {
			existingObjects = append(existingObjects, objectName)
		}
	}

	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("객체 순회 중 오류: %v", err)
	}

	if len(existingObjects) == 0 {
		return make(map[string][]int), nil
	}

	// 병렬 처리로 매핑 수집
	type MappingResult struct {
		PG   string
		OSDs []int
		Err  error
	}

	objectChan := make(chan string, len(existingObjects))
	resultChan := make(chan MappingResult, len(existingObjects))
	var wg sync.WaitGroup

	// 8워커로 고정 (최적 성능)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for objectName := range objectChan {
				pgID, err := getObjectPGMapping(conn, poolName, objectName)
				if err != nil {
					resultChan <- MappingResult{Err: err}
					continue
				}

				osds, err := getPGOSDMapping(conn, pgID)
				if err != nil {
					resultChan <- MappingResult{Err: err}
					continue
				}

				resultChan <- MappingResult{PG: pgID, OSDs: osds}
			}
		}()
	}

	// 객체 분배
	go func() {
		for _, obj := range existingObjects {
			objectChan <- obj
		}
		close(objectChan)
	}()

	// 결과 수집
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	pgToOSDs := make(map[string][]int)
	for result := range resultChan {
		if result.Err == nil {
			pgToOSDs[result.PG] = result.OSDs
		}
	}

	return pgToOSDs, nil
}
