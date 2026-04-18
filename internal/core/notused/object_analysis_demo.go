package notused

import (
	"fmt"
	"strings"

	"github.com/ceph/go-ceph/rados"
	"github.com/ceph/go-ceph/rbd"
)

// DemoObjectNumberCalculation 함수: RBD 객체 번호 계산 방식 설명
// 핵심: 객체 크기는 동적이며 이미지별로 다를 수 있음!
func DemoObjectNumberCalculation(conn *rados.Conn, poolName, imageName string) error {
	slog.Infof("=== RBD 객체 번호 계산 방식 데모 === [pool: %s, image: %s]", poolName, imageName)

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

	// 핵심: 이미지 메타데이터에서 동적으로 객체 크기 가져오기
	stat, err := image.Stat()
	if err != nil {
		return fmt.Errorf("이미지 정보 가져오기 실패: %v", err)
	}

	// 이미지별 고유 정보 출력
	slog.Infof("=== 이미지 메타데이터 (동적 정보) ===")
	slog.Infof("이미지 크기: %d bytes (%.2f MB)", stat.Size, float64(stat.Size)/1024/1024)
	slog.Infof("객체 크기: %d bytes (%.2f MB)", stat.Obj_size, float64(stat.Obj_size)/1024/1024)
	slog.Infof("총 객체 수: %d", stat.Num_objs)
	slog.Infof("Block Name Prefix: %s", stat.Block_name_prefix)

	// 수학적 계산 검증
	calculatedObjects := (stat.Size + stat.Obj_size - 1) / stat.Obj_size
	slog.Infof("=== 수학적 계산 검증 ===")
	slog.Infof("계산 공식: (이미지크기 + 객체크기 - 1) ÷ 객체크기")
	slog.Infof("계산 과정: 이미지크기 %d bytes, 객체크기 %d bytes, 계산된객체수 %d", stat.Size, stat.Obj_size, calculatedObjects)
	slog.Infof("Ceph 보고값: %d", stat.Num_objs)
	slog.Infof("계산 일치: %v", calculatedObjects == stat.Num_objs)

	// 실제 생성되는 객체 이름들
	slog.Infof("=== 실제 생성되는 객체 이름들 ===")
	slog.Infof("객체 번호 범위: %d ~ %d (총 %d개)", 0, stat.Num_objs-1, stat.Num_objs)

	// 처음 5개와 마지막 1개 객체 이름 예시
	for i := uint64(0); i < 5 && i < stat.Num_objs; i++ {
		objectName := fmt.Sprintf("%s.%016x", stat.Block_name_prefix, i)
		slog.Infof("객체 이름 예시: %s (번호: %d)", objectName, i)
	}

	if stat.Num_objs > 5 {
		slog.Infof("중간 객체 생략: %d개", stat.Num_objs-6)
		lastIdx := stat.Num_objs - 1
		objectName := fmt.Sprintf("%s.%016x", stat.Block_name_prefix, lastIdx)
		slog.Infof("마지막 객체: %s (번호: %d)", objectName, lastIdx)
	}

	// 효율성 비교
	slog.Infof("=== 효율성 비교 ===")
	totalPossible := uint64(1) << 63 // 2^63 to avoid overflow
	slog.Infof("가능한 모든 16진수 조합: %d (약 922경개)", totalPossible)
	slog.Infof("실제 확인 필요: %d", stat.Num_objs)

	if stat.Num_objs > 0 {
		efficiency := float64(totalPossible) / float64(stat.Num_objs)
		slog.Infof("효율성 개선: %.2f배 빨라짐", efficiency)
	}

	// 다양한 객체 크기 시나리오
	slog.Infof("=== 객체 크기가 중요한 이유 ===")
	slog.Infof("RBD 이미지는 생성 시 다양한 객체 크기를 가질 수 있습니다:")
	slog.Infof("기본값: 4MB (4194304 bytes)")
	slog.Infof("가능한 값: 4KB, 64KB, 1MB, 2MB, 4MB, 8MB, 16MB, 32MB")
	slog.Infof("현재 이미지 객체 크기: %d bytes (%.2f MB)", stat.Obj_size, float64(stat.Obj_size)/1024/1024)
	slog.Infof("정적 4MB 가정은 틀릴 수 있습니다!")

	return nil
}

// DemoObjectMapping 함수: 실제 객체 매핑 과정 단계별 시연
func DemoObjectMapping(conn *rados.Conn, poolName, imageName string) error {
	slog.Infof("=== RBD 객체 매핑 과정 데모 === [pool: %s, image: %s]", poolName, imageName)

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

	// 동적 메타데이터 조회
	stat, err := image.Stat()
	if err != nil {
		return fmt.Errorf("이미지 정보 가져오기 실패: %v", err)
	}

	// 분석할 객체 수 제한 (데모용)
	maxDemo := 3
	if stat.Num_objs < uint64(maxDemo) {
		maxDemo = int(stat.Num_objs)
	}

	header := fmt.Sprintf("%-5s %-35s %-10s %-15s %-20s", "번호", "객체 이름", "존재여부", "PG", "OSDs")
	slog.Infof("단계별 객체 분석 (최대 %d개)", maxDemo)
	slog.Infof(header)
	slog.Infof(strings.Repeat("-", 85))

	for i := 0; i < maxDemo; i++ {
		// 동적 prefix 사용
		objectName := fmt.Sprintf("%s.%016x", stat.Block_name_prefix, uint64(i))

		// 객체 존재 확인
		exists := "❌"
		_, err := ioctx.Stat(objectName)
		if err == nil {
			exists = "✅"
		}

		// PG 매핑 조회
		pgid := "N/A"
		osds := "N/A"

		if exists == "✅" {
			if pg, err := getObjectPGMapping(conn, poolName, objectName); err == nil {
				pgid = pg
				if osdList, err := getPGOSDMapping(conn, pg); err == nil {
					osds = fmt.Sprintf("%v", osdList)
				}
			}
		}

		row := fmt.Sprintf("%-5d %-35s %-10s %-15s %-20s", i, objectName, exists, pgid, osds)
		slog.Infof(row)
	}

	if stat.Num_objs > uint64(maxDemo) {
		slog.Infof("객체 상세 정보: 총 %d개, 표시 %d개", stat.Num_objs, maxDemo)
	}

	return nil
}

// DemoObjectSizeComparison 함수: 다양한 객체 크기 시나리오 비교
func DemoObjectSizeComparison() {
	slog.Infof("=== 객체 크기별 시나리오 비교 ===")

	// 동일한 100MB 이미지, 다른 객체 크기들
	imageSize := uint64(100 * 1024 * 1024) // 100MB
	objectSizes := []uint64{
		4 * 1024,         // 4KB
		64 * 1024,        // 64KB
		1024 * 1024,      // 1MB
		4 * 1024 * 1024,  // 4MB (기본값)
		8 * 1024 * 1024,  // 8MB
		32 * 1024 * 1024, // 32MB
	}

	header := fmt.Sprintf("%-10s %-15s %-15s %-20s", "객체크기", "총 객체수", "마지막객체크기", "효율성")
	slog.Infof("동일한 %dMB 이미지의 객체 크기별 분할:", imageSize/1024/1024)
	slog.Infof(header)
	slog.Infof(strings.Repeat("-", 65))

	for _, objSize := range objectSizes {
		totalObjects := (imageSize + objSize - 1) / objSize
		lastObjectSize := imageSize % objSize
		if lastObjectSize == 0 {
			lastObjectSize = objSize
		}

		efficiency := "완전활용"
		efficiencyPercent := 100.0
		if lastObjectSize < objSize {
			efficiencyPercent = float64(lastObjectSize) / float64(objSize) * 100
			efficiency = fmt.Sprintf("%.1f%% 활용", efficiencyPercent)
		}

		row := fmt.Sprintf("%-10s %-15d %-15s %-20s",
			formatSize(objSize), totalObjects, formatSize(lastObjectSize), efficiency)

		slog.Infof(row)
	}

	slog.Infof("핵심 포인트:")
	slog.Infof("- 객체 크기가 클수록 총 객체 수는 적어짐")
	slog.Infof("- 마지막 객체는 부분적으로만 사용될 수 있음")
	slog.Infof("- 따라서 정적 4MB 가정은 완전히 틀릴 수 있음!")
}

// formatSize 함수: 바이트를 읽기 쉬운 형태로 변환
func formatSize(bytes uint64) string {
	if bytes >= 1024*1024 {
		return fmt.Sprintf("%dMB", bytes/1024/1024)
	} else if bytes >= 1024 {
		return fmt.Sprintf("%dKB", bytes/1024)
	}
	return fmt.Sprintf("%dB", bytes)
}
