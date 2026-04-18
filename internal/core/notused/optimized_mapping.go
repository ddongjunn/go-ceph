package notused

import (
	"fmt"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ceph/go-ceph/rados"
	"github.com/ceph/go-ceph/rbd"
)

// OptimizedRBDMapping 함수: 동적 객체 크기를 고려한 최적화된 RBD 매핑 분석
// 핵심: 정적 4MB 가정 대신 이미지 메타데이터에서 동적으로 객체 크기 조회
func OptimizedRBDMapping(conn *rados.Conn, poolName, imageName string, sampleSize int) error {
	slog.Infof(" === 최적화된 RBD 매핑 분석 (동적 객체 크기) === [pool: %s, image: %s]", poolName, imageName)

	// 핵심 1: 동적 이미지 메타데이터 조회 (정적 4MB 가정 X)
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

	slog.Info("\n === 동적 이미지 메타데이터 ===")
	slog.Infof("이미지 크기: %d bytes (%.2f MB)", stat.Size, float64(stat.Size)/1024/1024)
	slog.Infof("객체 크기 (동적 조회): %d bytes (%.2f MB)", stat.Obj_size, float64(stat.Obj_size)/1024/1024)
	slog.Infof("총 객체 수 (정확한 계산): %d", stat.Num_objs)
	slog.Infof("Block Prefix (이미지별 고유): %s", stat.Block_name_prefix)

	// 핵심 2: 정확한 객체 번호 범위 계산 (무차별 대입 아님!)
	totalObjects := int(stat.Num_objs)
	if totalObjects == 0 {
		slog.Info("객체가 없습니다. 데이터를 먼저 써주세요.")
		return nil
	}

	// 핵심 3: 샘플링 전략 (선택적)
	analyzeCount := totalObjects
	if sampleSize > 0 && sampleSize < totalObjects {
		analyzeCount = sampleSize
		slog.Infof("샘플링 전략 - 총 객체: %d, 분석할 객체: %d, 샘플 비율: %.2f%%",
			totalObjects, analyzeCount, float64(analyzeCount)/float64(totalObjects)*100)
	} else {
		slog.Infof("전체 분석 - 총 객체: %d", totalObjects)
	}

	slog.Info("\n === 객체 존재 확인 및 매핑 분석 ===")
	header := fmt.Sprintf("%-5s %-40s %-8s %-12s %-15s", "번호", "객체 이름", "존재", "PG", "OSDs")
	slog.Info(header)
	separator := strings.Repeat("-", 85)
	slog.Info(separator)

	pgCount := make(map[string]int)
	osdSet := make(map[int]bool)
	existingObjects := 0

	// 분석할 객체 인덱스 결정
	var objectIndices []int
	if analyzeCount == totalObjects {
		// 전체 분석
		for i := 0; i < totalObjects; i++ {
			objectIndices = append(objectIndices, i)
		}
	} else {
		// 랜덤 샘플링
		rand.Seed(time.Now().UnixNano())
		indices := rand.Perm(totalObjects)
		objectIndices = indices[:analyzeCount]
		sort.Ints(objectIndices)
	}

	// 객체별 분석
	for _, i := range objectIndices {
		objectName := fmt.Sprintf("%s.%016x", stat.Block_name_prefix, uint64(i))

		// 객체 존재 확인
		exists := "❌"
		_, err := ioctx.Stat(objectName)
		if err == nil {
			exists = "✅"
			existingObjects++
		}

		// PG 매핑 조회 (존재하는 객체만)
		pgid := "N/A"
		osds := "N/A"
		if exists == "✅" {
			if pg, err := getObjectPGMapping(conn, poolName, objectName); err == nil {
				pgid = pg
				pgCount[pg]++
				if osdList, err := getPGOSDMapping(conn, pg); err == nil {
					osds = fmt.Sprintf("%v", osdList)
					for _, osd := range osdList {
						osdSet[osd] = true
					}
				}
			}
		}

		// 결과 출력
		row := fmt.Sprintf("%-5d %-40s %-8s %-12s %-15s", i, objectName, exists, pgid, osds)
		slog.Info(row)
	}

	// 요약 정보
	slog.Info("\n === 분석 결과 요약 ===")
	slog.Infof("분석 결과 - 총 객체: %d, 존재하는 객체: %d, PG 수: %d, OSD 수: %d",
		len(objectIndices), existingObjects, len(pgCount), len(osdSet))

	return nil
}

// 정적 vs 동적 접근 방식 비교 데모
func CompareStaticVsDynamic(conn *rados.Conn, poolName, imageName string) error {
	slog.Infof(" === 정적 vs 동적 접근 방식 비교 ===")

	// IOContext 생성
	ioctx, err := conn.OpenIOContext(poolName)
	if err != nil {
		return fmt.Errorf("IOContext 열기 실패: %v", err)
	}
	defer ioctx.Destroy()

	// RBD 이미지 열기
	image := rbd.GetImage(ioctx, imageName)
	err = image.Open()
	if err != nil {
		return fmt.Errorf("이미지 열기 실패: %v", err)
	}
	defer image.Close()

	// 이미지 정보 조회
	stat, err := image.Stat()
	if err != nil {
		return fmt.Errorf("이미지 정보 가져오기 실패: %v", err)
	}

	slog.Info("\n === 잘못된 정적 접근 방식 ===")
	staticObjectSize := uint64(4 * 1024 * 1024) // 4MB 고정 가정
	staticTotalObjects := (stat.Size + staticObjectSize - 1) / staticObjectSize
	slog.Infof("가정한 객체 크기: %d bytes (%.2f MB)", staticObjectSize, float64(staticObjectSize)/1024/1024)
	slog.Infof("계산된 객체 수: %d", staticTotalObjects)
	slog.Info("문제점: 실제와 다를 수 있음!")

	slog.Info("\n === 올바른 동적 접근 방식 ===")
	slog.Infof("실제 객체 크기: %d bytes (%.2f MB)", stat.Obj_size, float64(stat.Obj_size)/1024/1024)
	slog.Infof("실제 객체 수: %d", stat.Num_objs)
	slog.Info("장점: 항상 정확함!")

	slog.Info("\n === 비교 결과 ===")
	if staticTotalObjects == stat.Num_objs {
		slog.Info("이번 경우: 우연히 일치 (객체 크기가 4MB)")
		slog.Info("하지만 다른 이미지에서는 틀릴 수 있음!")
	} else {
		slog.Infof("정적 방식 오차 - 오차: %d, 오차 비율: %.2f%%",
			int64(staticTotalObjects)-int64(stat.Num_objs),
			float64(int64(staticTotalObjects)-int64(stat.Num_objs))/float64(stat.Num_objs)*100)
		slog.Info("동적 방식이 필수적임을 증명! ")
	}

	return nil
}

// 사용자가 원하는 최종 결과: 이미지의 PG/OSD 매핑 요약
func AnalyzeRBDImageMapping(conn *rados.Conn, poolName, imageName string) error {
	slog.Infof("\n=== RBD 이미지 PG/OSD 매핑 분석 === ")

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

	slog.Info("= 이미지 기본 정보")
	slog.Infof("풀: %s, 이미지: %s", poolName, imageName)
	slog.Infof("크기 - %.2f MB (%d bytes)", float64(stat.Size)/1024/1024, stat.Size)
	slog.Infof("객체 크기 - %.2f MB (%d bytes)", float64(stat.Obj_size)/1024/1024, stat.Obj_size)
	slog.Infof("총 객체 수 - %d", stat.Num_objs)

	// 2. 실제 존재하는 객체들의 PG/OSD 매핑 수집
	pgToOSDs := make(map[string][]int)
	allOSDs := make(map[int]bool)
	existingObjects := 0

	slog.Info("\n === 실제 존재하는 객체 분석 중... ===")

	for objIdx := 0; objIdx < int(stat.Num_objs); objIdx++ {
		objectName := fmt.Sprintf("%s.%016x", stat.Block_name_prefix, uint64(objIdx))

		// 객체 존재 확인
		_, err := ioctx.Stat(objectName)
		if err != nil {
			continue // 객체가 존재하지 않음
		}
		existingObjects++

		// PG 매핑 조회
		pgID, err := getObjectPGMapping(conn, poolName, objectName)
		if err != nil {
			slog.Warnf("PG 매핑 조회 실패 - 객체: %s, 오류: %v", objectName, err)
			continue
		}

		// OSD 매핑 조회
		osds, err := getPGOSDMapping(conn, pgID)
		if err != nil {
			slog.Warnf("OSD 매핑 조회 실패 - PG: %s, 오류: %v", pgID, err)
			continue
		}

		// 결과 수집
		pgToOSDs[pgID] = osds
		for _, osd := range osds {
			allOSDs[osd] = true
		}

		// 진행상황 표시 (매 10개마다)
		if existingObjects%10 == 0 || existingObjects <= 5 {
			slog.Infof("진행 - 존재하는 객체: %d", existingObjects)
		}
	}

	// 3. 최종 결과 출력
	slog.Info("\n === 최종 매핑 결과 ===")
	slog.Infof("분석한 총 객체 - %d", stat.Num_objs)
	slog.Infof("실제 존재하는 객체 - %d (%.2f%%)", existingObjects, float64(existingObjects)/float64(stat.Num_objs)*100)
	slog.Infof("사용된 PG 수 - %d", len(pgToOSDs))
	slog.Infof("사용된 OSD 수 - %d", len(allOSDs))

	// 4. PG별 상세 매핑 정보
	slog.Info("\n === PG → OSD 매핑 상세 ===")
	slog.Info(fmt.Sprintf("%-15s %-10s %s", "PG", "객체수", "OSDs"))
	slog.Info(fmt.Sprintf("%-15s %-10s %s", "---------------", "----------", "------------------------"))

	// PG를 정렬하여 출력
	var pgList []string
	for pg := range pgToOSDs {
		pgList = append(pgList, pg)
	}
	sort.Strings(pgList)

	pgToObjectCount := make(map[string]int)
	for objIdx := 0; objIdx < int(stat.Num_objs); objIdx++ {
		objectName := fmt.Sprintf("%s.%016x", stat.Block_name_prefix, uint64(objIdx))
		if _, err := ioctx.Stat(objectName); err == nil {
			if pgID, err := getObjectPGMapping(conn, poolName, objectName); err == nil {
				pgToObjectCount[pgID]++
			}
		}
	}

	for _, pg := range pgList {
		osds := pgToOSDs[pg]
		objectCount := pgToObjectCount[pg]

		sort.Ints(osds)
		osdStrs := make([]string, len(osds))
		for i, osd := range osds {
			osdStrs[i] = fmt.Sprintf("%d", osd)
		}

		slog.Infof("%-15s %-10d [%s]", pg, objectCount, strings.Join(osdStrs, ", "))
	}

	// 5. OSD 사용 통계
	slog.Info("\n === OSD 사용 통계 ===")
	var osdList []int
	for osd := range allOSDs {
		osdList = append(osdList, osd)
	}
	sort.Ints(osdList)

	slog.Infof("사용된 OSD 목록 - %v", osdList)

	return nil
}

// 효율적인 실제 객체만 조회하는 함수
func AnalyzeRBDImageMappingEfficient(conn *rados.Conn, poolName, imageName string) error {
	slog.Infof("\n=== 효율적인 RBD 이미지 매핑 분석 (실제 객체만) ===")

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

	slog.Info("= 이미지 기본 정보")
	slog.Infof("풀: %s, 이미지: %s", poolName, imageName)
	slog.Infof("크기 - %.2f MB (%d bytes)", float64(stat.Size)/1024/1024, stat.Size)
	slog.Infof("객체 크기 - %.2f MB (%d bytes)", float64(stat.Obj_size)/1024/1024, stat.Obj_size)
	slog.Infof("총 객체 슬롯 - %d", stat.Num_objs)

	// 2. 🚀 핵심: 실제 존재하는 객체만 조회
	slog.Info("\n=== 실제 존재하는 객체 조회 중... ===")

	// RBD 객체 prefix로 필터링하여 실제 객체만 조회
	objectPrefix := stat.Block_name_prefix

	// RADOS에서 해당 prefix를 가진 객체들만 조회
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

	slog.Infof("발견된 실제 객체 수: %d (전체 슬롯의 %.2f%%)",
		len(existingObjects), float64(len(existingObjects))/float64(stat.Num_objs)*100)

	// 3. 실제 존재하는 객체들의 PG/OSD 매핑 분석
	pgToOSDs := make(map[string][]int)
	allOSDs := make(map[int]bool)
	pgToObjectCount := make(map[string]int)

	slog.Info("\n=== 실제 객체들의 매핑 분석 ===")

	for i, objectName := range existingObjects {
		// PG 매핑 조회
		pgID, err := getObjectPGMapping(conn, poolName, objectName)
		if err != nil {
			slog.Warnf("PG 매핑 조회 실패 - 객체: %s, 오류: %v", objectName, err)
			continue
		}

		// OSD 매핑 조회
		osds, err := getPGOSDMapping(conn, pgID)
		if err != nil {
			slog.Warnf("OSD 매핑 조회 실패 - PG: %s, 오류: %v", pgID, err)
			continue
		}

		// 결과 수집
		pgToOSDs[pgID] = osds
		pgToObjectCount[pgID]++
		for _, osd := range osds {
			allOSDs[osd] = true
		}

		// 진행상황 표시
		if (i+1)%5 == 0 || i < 5 {
			slog.Infof("진행 - 분석 완료: %d/%d", i+1, len(existingObjects))
		}
	}

	// 4. 최종 결과 출력
	slog.Info("\n=== 효율적 분석 결과 ===")
	slog.Infof("총 객체 슬롯 - %d", stat.Num_objs)
	slog.Infof("실제 존재하는 객체 - %d (%.2f%%)", len(existingObjects),
		float64(len(existingObjects))/float64(stat.Num_objs)*100)
	slog.Infof("사용된 PG 수 - %d", len(pgToOSDs))
	slog.Infof("사용된 OSD 수 - %d", len(allOSDs))
	slog.Infof("⚡ 성능 개선 - %d개 객체 스킵 (%.1f%% 효율성 향상)",
		int(stat.Num_objs)-len(existingObjects),
		float64(int(stat.Num_objs)-len(existingObjects))/float64(stat.Num_objs)*100)

	// 5. PG별 상세 매핑 정보
	slog.Info("\n=== PG → OSD 매핑 상세 ===")
	slog.Info(fmt.Sprintf("%-15s %-10s %s", "PG", "객체수", "OSDs"))
	slog.Info(fmt.Sprintf("%-15s %-10s %s", "---------------", "----------", "------------------------"))

	// PG를 정렬하여 출력
	var pgList []string
	for pg := range pgToOSDs {
		pgList = append(pgList, pg)
	}
	sort.Strings(pgList)

	for _, pg := range pgList {
		osds := pgToOSDs[pg]
		objectCount := pgToObjectCount[pg]

		sort.Ints(osds)
		osdStrs := make([]string, len(osds))
		for i, osd := range osds {
			osdStrs[i] = fmt.Sprintf("%d", osd)
		}

		slog.Infof("%-15s %-10d [%s]", pg, objectCount, strings.Join(osdStrs, ", "))
	}

	// 6. OSD 사용 통계
	slog.Info("\n=== OSD 사용 통계 ===")
	var osdList []int
	for osd := range allOSDs {
		osdList = append(osdList, osd)
	}
	sort.Ints(osdList)

	slog.Infof("사용된 OSD 목록 - %v", osdList)

	return nil
}

// 🚀 글로벌 PG→OSD 매핑 캐시 (함수 호출 간 재사용)
type PGCache struct {
	sync.RWMutex
	cache     map[string][]int
	maxSize   int // 최대 캐시 항목 수
	hitCount  int64
	missCount int64
}

var globalPGCache = &PGCache{
	cache:   make(map[string][]int),
	maxSize: 5000, // 최대 5,000개 PG (약 220KB, 안전한 크기)
}

// 캐시 정리 함수
func (c *PGCache) clearOldCache() {
	// 캐시가 가득 찬 경우 절반 삭제 (간단한 전략)
	if len(c.cache) >= c.maxSize {
		slog.Warnf("캐시 크기 제한 도달 (%d개), 절반 정리 중...", len(c.cache))

		// 맵의 절반을 무작위로 삭제 (간단하고 효과적)
		count := 0
		target := len(c.cache) / 2
		for key := range c.cache {
			delete(c.cache, key)
			count++
			if count >= target {
				break
			}
		}
		slog.Infof("캐시 정리 완료: %d개 → %d개", len(c.cache)+count, len(c.cache))
	}
}

// 캐시 통계 초기화
func (c *PGCache) resetStats() {
	c.Lock()
	c.hitCount = 0
	c.missCount = 0
	c.Unlock()
}

// 🧹 글로벌 캐시 완전 초기화 (테스트용)
func ClearGlobalPGCache() error {
	globalPGCache.Lock()
	defer globalPGCache.Unlock()

	// 캐시 맵 완전 초기화
	globalPGCache.cache = make(map[string][]int)
	globalPGCache.hitCount = 0
	globalPGCache.missCount = 0

	slog.Info("✅ 글로벌 PG 캐시 완전 초기화 완료")
	return nil
}

// 📊 글로벌 캐시 상태 조회
func GetGlobalPGCacheStats() (size int, hits int64, misses int64) {
	globalPGCache.RLock()
	defer globalPGCache.RUnlock()

	return len(globalPGCache.cache), globalPGCache.hitCount, globalPGCache.missCount
}

// 🚀 최고 성능: 병렬 처리 + 글로벌 캐싱 최적화된 RBD 매핑 분석
func AnalyzeRBDImageMappingParallel(conn *rados.Conn, poolName, imageName string, workerCount int) error {
	return AnalyzeRBDImageMappingParallelWithCache(conn, poolName, imageName, workerCount, true)
}

// 🚀 캐시 옵션을 포함한 병렬 처리 RBD 매핑 분석
func AnalyzeRBDImageMappingParallelWithCache(conn *rados.Conn, poolName, imageName string, workerCount int, useGlobalCache bool) error {
	slog.Infof("\n=== 🚀 병렬 처리 최적화 RBD 매핑 분석 === [워커: %d개, 글로벌캐시: %v]", workerCount, useGlobalCache)

	startTime := time.Now()

	// 캐시 선택
	var pgCache *PGCache
	if useGlobalCache {
		pgCache = globalPGCache
		slog.Infof("🔄 글로벌 캐시 사용 (기존 항목: %d개)", len(pgCache.cache))
	} else {
		pgCache = &PGCache{
			cache:   make(map[string][]int),
			maxSize: 5000,
		}
		slog.Info("🆕 새 로컬 캐시 생성")
	}

	// 통계 초기화 (이번 실행용)
	pgCache.resetStats()

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

	slog.Info("= 이미지 기본 정보")
	slog.Infof("풀: %s, 이미지: %s", poolName, imageName)
	slog.Infof("크기 - %.2f MB (%d bytes)", float64(stat.Size)/1024/1024, stat.Size)
	slog.Infof("객체 크기 - %.2f MB (%d bytes)", float64(stat.Obj_size)/1024/1024, stat.Obj_size)
	slog.Infof("총 객체 수 - %d", stat.Num_objs)
	slog.Infof("워커 수: %d개 (병렬 처리)", workerCount)

	// 2. 🚀 실제 존재하는 객체만 조회 (기존 최적화 유지)
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
	slog.Infof("객체 발견 완료: %d개 (%.2f%%, 소요시간: %v)",
		len(existingObjects), float64(len(existingObjects))/float64(stat.Num_objs)*100, objectDiscoveryTime)

	if len(existingObjects) == 0 {
		slog.Info("분석할 객체가 없습니다.")
		return nil
	}

	// 3. 🚀 병렬 처리를 위한 워커 풀 설정
	type MappingResult struct {
		ObjectName string
		PG         string
		OSDs       []int
		Error      error
	}

	// 채널 설정
	objectChan := make(chan string, len(existingObjects))
	resultChan := make(chan MappingResult, len(existingObjects))

	// 4. 🚀 워커 고루틴 시작
	var wg sync.WaitGroup
	mappingStart := time.Now()

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

				// 🚀 캐시된 OSD 매핑 확인
				pgCache.RLock()
				cachedOSDs, exists := pgCache.cache[pgID]
				pgCache.RUnlock()

				if exists {
					// 캐시 히트!
					result.OSDs = cachedOSDs
					pgCache.Lock()
					pgCache.hitCount++
					pgCache.Unlock()
				} else {
					// 캐시 미스 - 새로 조회
					osds, err := getPGOSDMapping(conn, pgID)
					if err != nil {
						result.Error = fmt.Errorf("OSD 매핑 실패: %v", err)
						resultChan <- result
						continue
					}
					result.OSDs = osds

					// 캐시에 저장
					pgCache.Lock()
					pgCache.cache[pgID] = osds
					pgCache.missCount++
					pgCache.Unlock()

					// 캐시 크기 제한 확인
					pgCache.clearOldCache()
				}

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

	slog.Info("\n=== 🚀 병렬 매핑 분석 진행 중... ===")

	for result := range resultChan {
		processedCount++

		if result.Error != nil {
			errorCount++
			if errorCount <= 5 { // 처음 5개 에러만 로깅
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

		// 진행상황 표시
		if processedCount%50 == 0 || processedCount <= 10 {
			slog.Infof("진행: %d/%d (%.1f%%) - 에러: %d개",
				processedCount, len(existingObjects),
				float64(processedCount)/float64(len(existingObjects))*100, errorCount)
		}
	}

	mappingTime := time.Since(mappingStart)
	totalTime := time.Since(startTime)

	// 7. 캐시 효율성 통계
	pgCache.RLock()
	cacheSize := len(pgCache.cache)
	hitCount := pgCache.hitCount
	missCount := pgCache.missCount
	pgCache.RUnlock()

	var hitRate float64
	if hitCount+missCount > 0 {
		hitRate = float64(hitCount) / float64(hitCount+missCount) * 100
	}

	slog.Infof("캐시 효율성: %.1f%% (고유 PG: %d개, 히트: %d, 미스: %d)", hitRate, cacheSize, hitCount, missCount)

	// 8. 최종 결과 출력
	slog.Info("\n=== 🚀 병렬 처리 분석 결과 ===")
	slog.Infof("총 처리 시간: %v", totalTime)
	slog.Infof("  - 객체 발견: %v (%.1f%%)", objectDiscoveryTime, float64(objectDiscoveryTime)/float64(totalTime)*100)
	slog.Infof("  - 병렬 매핑: %v (%.1f%%)", mappingTime, float64(mappingTime)/float64(totalTime)*100)
	slog.Infof("워커 수: %d개", workerCount)
	slog.Infof("처리 성공: %d개, 실패: %d개", processedCount-errorCount, errorCount)

	// PG별 분포
	slog.Info("\n=== PG별 객체 분포 ===")
	type PGInfo struct {
		PG    string
		Count int
		OSDs  []int
	}

	var pgInfos []PGInfo
	for pg, osds := range pgToOSDs {
		pgInfos = append(pgInfos, PGInfo{
			PG:    pg,
			Count: pgToObjectCount[pg],
			OSDs:  osds,
		})
	}

	// PG를 객체 수 기준으로 정렬
	sort.Slice(pgInfos, func(i, j int) bool {
		return pgInfos[i].Count > pgInfos[j].Count
	})

	for i, info := range pgInfos {
		if i < 10 { // 상위 10개만 상세 출력
			slog.Infof("PG %s: %d개 객체 → OSDs %v", info.PG, info.Count, info.OSDs)
		}
	}

	if len(pgInfos) > 10 {
		slog.Infof("... 외 %d개 PG", len(pgInfos)-10)
	}

	// OSD 사용 현황
	var osdList []int
	for osd := range allOSDs {
		osdList = append(osdList, osd)
	}
	sort.Ints(osdList)

	slog.Info("\n=== 최종 요약 ===")
	slog.Infof("사용 중인 PG: %d개", len(pgToOSDs))
	slog.Infof("사용 중인 OSD: %d개 %v", len(allOSDs), osdList)
	slog.Infof("평균 PG당 객체 수: %.1f개", float64(len(existingObjects))/float64(len(pgToOSDs)))

	return nil
}

// 개선된 스마트 샘플링 분석 (존재하는 객체 우선)
func AnalyzeRBDImageMappingWithSmartSampling(conn *rados.Conn, poolName, imageName string, sampleSize int) error {
	slog.Infof(" === RBD 이미지 스마트 샘플링 분석 === [pool: %s, image: %s, sample_size: %d]", poolName, imageName, sampleSize)

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

	slog.Info("\n === 이미지 기본 정보 ===")
	slog.Infof("이미지 - 풀: %s, 이름: %s", poolName, imageName)
	slog.Infof("크기 - %.2f MB (%d bytes)", float64(stat.Size)/1024/1024, stat.Size)
	slog.Infof("총 객체 수 - %d", stat.Num_objs)
	slog.Infof("샘플링 크기 - %d (%.2f%%)", sampleSize, float64(sampleSize)/float64(stat.Num_objs)*100)

	// 2. 스마트 샘플링: 존재하는 객체 우선 찾기
	slog.Info("\n === 스마트 샘플링 중... ===")

	existingObjects := []int{}
	checkedObjects := 0
	maxChecks := int(stat.Num_objs)
	if maxChecks > sampleSize*10 { // 최대 샘플 크기의 10배까지만 확인
		maxChecks = sampleSize * 10
	}

	rand.Seed(time.Now().UnixNano())
	indices := rand.Perm(int(stat.Num_objs))

	for _, objIdx := range indices {
		if len(existingObjects) >= sampleSize {
			break
		}
		if checkedObjects >= maxChecks {
			break
		}

		objectName := fmt.Sprintf("%s.%016x", stat.Block_name_prefix, uint64(objIdx))
		_, err := ioctx.Stat(objectName)
		checkedObjects++

		if err == nil {
			existingObjects = append(existingObjects, objIdx)
		}

		// 진행상황 갱신 (매 10개마다)
		if checkedObjects%10 == 0 {
			slog.Infof("진행 - 확인한 객체: %d, 최대 확인: %d, 존재하는 객체: %d", checkedObjects, maxChecks, len(existingObjects))
		}
	}

	if len(existingObjects) == 0 {
		slog.Info("샘플링 실패: 존재하는 객체를 찾지 못했습니다.")
		slog.Infof("   (총 %d개 객체 확인했지만 모두 비어있음)", checkedObjects)
		return nil
	}

	slog.Infof("샘플링 성공: %d개 존재하는 객체 발견 (총 %d개 확인)", len(existingObjects), checkedObjects)

	// 3. 샘플 객체들의 PG/OSD 매핑 분석
	pgToOSDs := make(map[string][]int)
	pgToObjectCount := make(map[string]int)
	allOSDs := make(map[int]bool)

	slog.Info("\n === 샘플 객체 매핑 분석 ===")

	for i, objIdx := range existingObjects {
		objectName := fmt.Sprintf("%s.%016x", stat.Block_name_prefix, uint64(objIdx))

		// PG 매핑 조회
		pgID, err := getObjectPGMapping(conn, poolName, objectName)
		if err != nil {
			slog.Warnf("PG 매핑 조회 실패 - 객체: %s, 오류: %v", objectName, err)
			continue
		}

		// OSD 매핑 조회
		osds, err := getPGOSDMapping(conn, pgID)
		if err != nil {
			slog.Warnf("OSD 매핑 조회 실패 - PG: %s, 오류: %v", pgID, err)
			continue
		}

		// 결과 수집
		pgToOSDs[pgID] = osds
		pgToObjectCount[pgID]++
		for _, osd := range osds {
			allOSDs[osd] = true
		}

		slog.Infof("[%d/%d] 객체 #%d → PG %s → OSDs %v", i+1, len(existingObjects), objIdx, pgID, osds)
	}

	// 4. 샘플링 기반 추정 결과
	estimatedTotalObjects := int(float64(len(existingObjects)) / float64(checkedObjects) * float64(stat.Num_objs))

	slog.Info("\n === 샘플링 기반 추정 결과 ===")
	slog.Infof("샘플 크기 - %d", len(existingObjects))
	slog.Infof("확인한 객체 - %d", checkedObjects)
	slog.Infof("존재 비율 - %.2f%%", float64(len(existingObjects))/float64(checkedObjects)*100)
	slog.Infof("추정 총 존재 객체 - %d", estimatedTotalObjects)
	slog.Infof("발견된 PG 수 - %d", len(pgToOSDs))
	slog.Infof("발견된 OSD 수 - %d", len(allOSDs))

	// 5. PG별 매핑 정보 (샘플 기준)
	slog.Info("\n === 발견된 PG → OSD 매핑 ===")
	slog.Info(fmt.Sprintf("%-15s %-10s %s", "PG", "샘플객체수", "OSDs"))
	slog.Info(fmt.Sprintf("%-15s %-10s %s", "---------------", "----------", "------------------------"))

	var pgList []string
	for pg := range pgToOSDs {
		pgList = append(pgList, pg)
	}
	sort.Strings(pgList)

	for _, pg := range pgList {
		osds := pgToOSDs[pg]
		objectCount := pgToObjectCount[pg]

		sort.Ints(osds)
		osdStrs := make([]string, len(osds))
		for i, osd := range osds {
			osdStrs[i] = fmt.Sprintf("%d", osd)
		}

		slog.Infof("%-15s %-10d [%s]", pg, objectCount, strings.Join(osdStrs, ", "))
	}

	// 6. OSD 사용 통계
	slog.Info("\n === OSD 사용 통계 ===")
	var osdList []int
	for osd := range allOSDs {
		osdList = append(osdList, osd)
	}
	sort.Ints(osdList)

	slog.Infof("발견된 OSD 목록 - %v", osdList)

	return nil
}

// 🚀 Iterator 병렬화 기반 RBD 매핑 분석 (객체 발견 + 매핑 동시 병렬화)
func AnalyzeRBDImageMappingParallelIterator(conn *rados.Conn, poolName, imageName string, workerCount int) error {
	slog.Infof("=== RBD 이미지 병렬 Iterator 분석 === [pool: %s, image: %s, workers: %d]", poolName, imageName, workerCount)

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
		return fmt.Errorf("이미지 정보 조회 실패: %v", err)
	}

	slog.Infof("이미지 정보: 크기=%d bytes, 객체크기=%d bytes, 총객체수=%d개",
		stat.Size, stat.Obj_size, stat.Num_objs)

	// 2. 글로벌 캐시 통계 초기화
	globalPGCache.resetStats()

	// 3. 워커별 객체 범위 분할
	objectsPerWorker := int(stat.Num_objs) / workerCount
	if objectsPerWorker == 0 {
		objectsPerWorker = 1
	}

	// 결과 수집용 채널
	type WorkerResult struct {
		WorkerID    int
		Objects     []string
		PGMappings  map[string][]int
		ProcessTime time.Duration
		ObjectCount int
		Error       error
	}

	resultChan := make(chan WorkerResult, workerCount)
	var wg sync.WaitGroup

	slog.Infof("워커별 객체 범위 분할: %d개씩 %d개 워커", objectsPerWorker, workerCount)

	// 4. 병렬 워커 실행 (각 워커가 Iterator + 매핑 동시 처리)
	for workerID := 0; workerID < workerCount; workerID++ {
		wg.Add(1)
		go func(wID int) {
			defer wg.Done()

			workerStart := time.Now()
			result := WorkerResult{
				WorkerID:   wID,
				Objects:    []string{},
				PGMappings: make(map[string][]int),
			}

			// 워커별 IOContext 생성 (스레드 안전성)
			workerIOCtx, err := conn.OpenIOContext(poolName)
			if err != nil {
				result.Error = fmt.Errorf("워커 %d IOContext 생성 실패: %v", wID, err)
				resultChan <- result
				return
			}
			defer workerIOCtx.Destroy()

			// 워커별 객체 범위 계산
			startObj := wID * objectsPerWorker
			endObj := startObj + objectsPerWorker
			if wID == workerCount-1 {
				endObj = int(stat.Num_objs) // 마지막 워커는 나머지 모두 처리
			}

			slog.Infof("워커 %d: 객체 범위 %d-%d 처리 시작", wID, startObj, endObj-1)

			// Iterator로 해당 범위의 객체 발견 + 즉시 매핑
			iter, err := workerIOCtx.Iter()
			if err != nil {
				result.Error = fmt.Errorf("워커 %d Iterator 생성 실패: %v", wID, err)
				resultChan <- result
				return
			}
			defer iter.Close()

			// 프리픽스 필터링으로 해당 워커 범위의 객체만 처리
			objectPrefix := stat.Block_name_prefix

			for iter.Next() {
				objectName := iter.Value()

				// 이 객체가 현재 워커 범위에 속하는지 확인
				if !strings.HasPrefix(objectName, objectPrefix) {
					continue
				}

				// 객체 번호 추출
				parts := strings.Split(objectName, ".")
				if len(parts) < 2 {
					continue
				}

				objNumHex := parts[len(parts)-1]
				objNum, err := strconv.ParseUint(objNumHex, 16, 64)
				if err != nil {
					continue
				}

				// 워커 범위 체크
				if int(objNum) < startObj || int(objNum) >= endObj {
					continue
				}

				// 객체 발견 즉시 PG/OSD 매핑 처리
				result.Objects = append(result.Objects, objectName)

				// PG 계산 - getObjectPGMapping 사용 (기존 병렬 처리와 동일)
				pgID, err := getObjectPGMapping(conn, poolName, objectName)
				if err != nil {
					slog.Warnf("워커 %d: PG 매핑 실패 %s: %v", wID, objectName, err)
					continue
				}

				// 글로벌 캐시에서 OSD 조회 (스레드 안전)
				globalPGCache.RLock()
				osds, cached := globalPGCache.cache[pgID]
				if cached {
					globalPGCache.hitCount++
				} else {
					globalPGCache.missCount++
				}
				globalPGCache.RUnlock()

				if !cached {
					// 캐시 미스: 실제 조회 후 캐시에 저장
					osds, err = getPGOSDMapping(conn, pgID)
					if err != nil {
						slog.Warnf("워커 %d: PG %s OSD 매핑 조회 실패: %v", wID, pgID, err)
						continue
					}

					// 글로벌 캐시에 저장 (스레드 안전)
					globalPGCache.Lock()
					globalPGCache.clearOldCache() // 크기 제한 확인
					globalPGCache.cache[pgID] = osds
					globalPGCache.Unlock()
				}

				result.PGMappings[pgID] = osds
				result.ObjectCount++
			}

			if err := iter.Err(); err != nil {
				result.Error = fmt.Errorf("워커 %d Iterator 오류: %v", wID, err)
				resultChan <- result
				return
			}

			result.ProcessTime = time.Since(workerStart)
			slog.Infof("워커 %d 완료: %d개 객체 처리, 소요시간: %v",
				wID, result.ObjectCount, result.ProcessTime)

			resultChan <- result
		}(workerID)
	}

	// 5. 모든 워커 완료 대기
	wg.Wait()
	close(resultChan)

	// 6. 결과 수집 및 통합
	allObjects := []string{}
	allPGMappings := make(map[string][]int)
	pgToObjectCount := make(map[string]int)
	allOSDs := make(map[int]bool)
	totalProcessTime := time.Duration(0)
	totalErrors := 0

	for result := range resultChan {
		if result.Error != nil {
			slog.Errorf("워커 %d 오류: %v", result.WorkerID, result.Error)
			totalErrors++
			continue
		}

		allObjects = append(allObjects, result.Objects...)
		totalProcessTime += result.ProcessTime

		for pgID, osds := range result.PGMappings {
			allPGMappings[pgID] = osds
			pgToObjectCount[pgID]++

			for _, osd := range osds {
				allOSDs[osd] = true
			}
		}
	}

	totalTime := time.Since(startTime)
	avgWorkerTime := totalProcessTime / time.Duration(workerCount)

	// 7. 캐시 통계 출력
	globalPGCache.RLock()
	hitRate := float64(globalPGCache.hitCount) / float64(globalPGCache.hitCount+globalPGCache.missCount) * 100
	cacheSize := len(globalPGCache.cache)
	globalPGCache.RUnlock()

	slog.Info("\n=== 병렬 Iterator 분석 결과 ===")
	slog.Infof("총 처리 시간: %v", totalTime)
	slog.Infof("평균 워커 시간: %v", avgWorkerTime)
	slog.Infof("병렬화 효율성: %.1f%% (이상적: %.1f%%)",
		float64(avgWorkerTime)/float64(totalTime)*100, 100.0/float64(workerCount)*100)

	if totalErrors > 0 {
		slog.Warnf("워커 오류: %d개", totalErrors)
	}

	slog.Infof("발견된 객체: %d개 (%.2f%% 사용률)",
		len(allObjects), float64(len(allObjects))/float64(stat.Num_objs)*100)
	slog.Infof("PG 캐시 통계: 히트율 %.1f%% (%d/%d), 캐시 크기: %d개",
		hitRate, globalPGCache.hitCount, globalPGCache.hitCount+globalPGCache.missCount, cacheSize)

	// 8. PG별 분포 출력 (기존과 동일)
	slog.Info("\n=== PG별 객체 분포 ===")
	type PGInfo struct {
		PG    string
		Count int
		OSDs  []int
	}

	var pgInfos []PGInfo
	for pg, osds := range allPGMappings {
		pgInfos = append(pgInfos, PGInfo{
			PG:    pg,
			Count: pgToObjectCount[pg],
			OSDs:  osds,
		})
	}

	sort.Slice(pgInfos, func(i, j int) bool {
		return pgInfos[i].Count > pgInfos[j].Count
	})

	for i, info := range pgInfos {
		if i < 10 {
			slog.Infof("PG %s: %d개 객체 → OSDs %v", info.PG, info.Count, info.OSDs)
		}
	}

	if len(pgInfos) > 10 {
		slog.Infof("... 외 %d개 PG", len(pgInfos)-10)
	}

	// 9. 최종 요약
	var osdList []int
	for osd := range allOSDs {
		osdList = append(osdList, osd)
	}
	sort.Ints(osdList)

	slog.Info("\n=== 최종 요약 ===")
	slog.Infof("사용 중인 PG: %d개", len(allPGMappings))
	slog.Infof("사용 중인 OSD: %d개 %v", len(allOSDs), osdList)
	slog.Infof("평균 PG당 객체 수: %.1f개", float64(len(allObjects))/float64(len(allPGMappings)))

	return nil
}
