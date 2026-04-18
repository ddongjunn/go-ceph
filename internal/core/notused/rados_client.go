package notused

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/ceph/go-ceph/rados"
	"github.com/ceph/go-ceph/rbd"
)

func GetClusterFSID() string {
	conn, err := rados.NewConn()
	if err != nil {
		slog.Errorf("Ceph 연결 실패: %v", err)
		return ""
	}

	err = conn.ReadConfigFile("/etc/notused/notused.conf")
	if err != nil {
		slog.Errorf("Ceph 설정 파일 읽기 실패: %v", err)
		return ""
	}

	err = conn.Connect()
	if err != nil {
		slog.Errorf("Ceph 클러스터 연결 실패: %v", err)
		return ""
	}
	defer conn.Shutdown()

	// FSID 출력
	fsid, err := conn.GetFSID()
	if err != nil {
		slog.Errorf("FSID 가져오기 실패: %v", err)
		return ""
	}
	slog.Infof("fsid: %s", fsid)

	// MgrCommand
	command, err := json.Marshal(
		map[string]string{"prefix": "get_command_descriptions", "format": "json"})
	buf, _, err := conn.MgrCommand([][]byte{command})

	var message map[string]interface{}
	err = json.Unmarshal(buf, &message)
	//slog.Info("MgrCommand", zap.String("command", string(buf)))

	// PGCommand
	pgid := "1.1"
	command, err = json.Marshal(
		map[string]string{"prefix": "query", "pgid": pgid, "format": "json"})
	buf, _, err = conn.PGCommand([]byte(pgid), [][]byte{command})

	var message2 map[string]interface{}
	err = json.Unmarshal(buf, &message2)
	//slog.Info("PGCommand", zap.String("command", string(buf)))

	// OSDCommand
	command, err = json.Marshal(
		map[string]string{"prefix": "get_command_descriptions", "format": "json"})
	if err != nil {
	}

	cmd, _ := json.Marshal(
		map[string]string{"prefix": "pg dump", "format": "json"})
	buf, _, err = conn.MgrCommand([][]byte{cmd})
	if err != nil {
		slog.Errorf("Error: %v", err)
	}
	err = json.Unmarshal(buf, &message)
	fmt.Printf("json: %s\n", buf)

	return fsid
}

func PrintPgDump(conn *rados.Conn) {
	cmd, _ := json.Marshal(map[string]string{
		"prefix": "pg dump",
		"format": "json",
	})

	buf, _, err := conn.MgrCommand([][]byte{cmd})
	if err != nil {
		slog.Errorf("MgrCommand[pgDump]: %v", err)
	}

	var prettyJSON bytes.Buffer
	err = json.Indent(&prettyJSON, buf, "", "  ")
	if err != nil {
		slog.Errorf("json.Indent: %v", err)
		slog.Errorf("prettyJSON: %s", buf)
		return
	}

	fmt.Println(prettyJSON.String())
}

func ListPoolsAndImages(conn *rados.Conn) (map[string][]string, error) {
	pools, err := conn.ListPools()
	if err != nil {
		return nil, fmt.Errorf("ListPools error: %w", err)
	}

	result := make(map[string][]string)
	for _, pool := range pools {
		ioctx, err := conn.OpenIOContext(pool)
		if err != nil {
			slog.Errorf("풀 열기 실패: %v", err)
			continue
		}

		images, err := rbd.GetImageNames(ioctx)
		ioctx.Destroy()
		if err != nil {
			slog.Errorf("풀 에서 이미지 목록 조회 실패: %v", err)
			continue
		}
		result[pool] = images
	}

	fmt.Println(result)
	return result, nil
}

func GetPoolIdByName(conn *rados.Conn, poolName string) int64 {
	id, err := conn.GetPoolByName(poolName)
	if err != nil {
	}
	return id
}

func GetPools(conn *rados.Conn) ([]string, error) {
	pools, err := conn.ListPools()
	if err != nil {
		return nil, fmt.Errorf("ListPools error: %w", err)
	}

	return pools, nil
}

func GetImages(conn *rados.Conn, poolName string) ([]map[string]interface{}, error) {
	ioctx, err := conn.OpenIOContext(poolName)
	if err != nil {
		return nil, err
	}
	defer ioctx.Destroy()

	imageNames, err := rbd.GetImageNames(ioctx)
	if err != nil {
		return nil, err
	}

	var results []map[string]interface{}

	for _, name := range imageNames {
		img, err := rbd.OpenImage(ioctx, name, rbd.NoSnapshot)
		if err != nil {
			continue
		}

		stat, err := img.Stat()
		if err != nil {
			img.Close()
			continue
		}

		results = append(results, map[string]interface{}{
			"name":     name,
			"size":     stat.Size,
			"obj_size": stat.Obj_size,
			"num_objs": stat.Num_objs,
		})

		img.Close()
	}

	return results, nil
}

func PrintMgrCommand(conn *rados.Conn) {
	cmd := map[string]interface{}{
		"prefix": "get_command_descriptions",
		"format": "json",
	}
	jsonCmd, err := json.Marshal(cmd)
	if err != nil {
		slog.Errorf("marshal command error: %v", err)
	}

	buf, _, err := conn.MgrCommand([][]byte{jsonCmd})
	if err != nil {
		slog.Errorf("MgrCommand[get_command_descriptions]: %v", err)
	}

	var pretty bytes.Buffer
	json.Indent(&pretty, buf, "", "  ")
	fmt.Println(pretty.String())
}

func MapRbdImageToPGAndOSD(conn *rados.Conn, poolName, imageName string) {
	fmt.Printf("\n=== RBD 이미지 매핑 분석 ===\n")
	fmt.Printf("Pool: %s\n", poolName)
	fmt.Printf("Image: %s\n", imageName)
	fmt.Printf("%s\n", repeat("-", 50))

	// 1. IOContext 생성
	ioctx, err := conn.OpenIOContext(poolName)
	if err != nil {
		slog.Errorf("IOContext 열기 실패: %v", err)
		return
	}
	defer ioctx.Destroy()

	// 2. RBD 이미지 열기
	image := rbd.GetImage(ioctx, imageName)
	err = image.Open()
	if err != nil {
		slog.Errorf("이미지 열기 실패: %v", err)
		return
	}
	defer image.Close()

	// 3. 이미지 정보 가져오기
	info, err := image.Stat()
	if err != nil {
		slog.Errorf("이미지 정보 가져오기 실패: %v", err)
		return
	}

	prefix := info.Block_name_prefix
	fmt.Printf("Object Prefix: %s\n", prefix)
	fmt.Printf("Image Size: %d bytes\n", info.Size)
	fmt.Printf("Object Size: %d bytes\n", info.Obj_size)
	fmt.Printf("Expected Objects: %d\n\n", (info.Size+info.Obj_size-1)/info.Obj_size)

	// 4. Pool의 모든 객체 나열하고 RBD 객체 필터링
	var rbdObjects []string
	var allObjects []string
	iter, err := ioctx.Iter()
	if err != nil {
		slog.Errorf("Iterator 생성 실패: %v", err)
		return
	}
	defer iter.Close()

	fmt.Printf("Pool에서 객체 검색 중... (prefix: %s)\n", prefix)

	for iter.Next() {
		objName := iter.Value()
		allObjects = append(allObjects, objName)

		// prefix 매칭 로직 개선
		if len(objName) >= len(prefix) && objName[:len(prefix)] == prefix {
			rbdObjects = append(rbdObjects, objName)
		}
	}

	if err := iter.Err(); err != nil {
		slog.Errorf("Iterator 오류: %v", err)
		return
	}

	fmt.Printf("Pool의 전체 객체 수: %d\n", len(allObjects))

	// 디버깅: 처음 10개 객체 이름 출력
	if len(allObjects) > 0 {
		fmt.Printf("Pool의 객체 예시 (처음 10개):\n")
		maxShow := len(allObjects)
		if maxShow > 10 {
			maxShow = 10
		}
		for i := 0; i < maxShow; i++ {
			fmt.Printf("  - %s\n", allObjects[i])
		}
	}

	if len(rbdObjects) == 0 {
		fmt.Printf("\n경고: prefix '%s'로 시작하는 객체를 찾을 수 없습니다.\n", prefix)

		// RBD 이미지에 데이터를 쓰는 테스트 제안
		fmt.Printf("\n해결 방법:\n")
		fmt.Printf("1. RBD 이미지에 데이터를 써서 객체를 생성하세요:\n")
		fmt.Printf("   rbd map %s/%s\n", poolName, imageName)
		fmt.Printf("   dd if=/dev/zero of=/dev/rbd0 bs=1M count=10\n")
		fmt.Printf("   rbd unmap /dev/rbd0\n")
		fmt.Printf("\n2. 또는 다른 RBD 이미지를 테스트해보세요.\n")

		// 다른 RBD 이미지들 찾기
		fmt.Printf("\n다른 RBD 객체 검색 중...\n")
		rbdPrefixes := make(map[string]int)
		for _, obj := range allObjects {
			if len(obj) > 8 && obj[:8] == "rbd_data" {
				// rbd_data.xxxxx 형태에서 prefix 추출
				parts := obj[:20] // rbd_data.16진수 부분만
				if len(parts) > 8 {
					rbdPrefixes[parts]++
				}
			}
		}

		if len(rbdPrefixes) > 0 {
			fmt.Printf("발견된 다른 RBD prefix들:\n")
			for prefix, count := range rbdPrefixes {
				fmt.Printf("  - %s (객체 수: %d)\n", prefix, count)
			}
		}

		return
	}

	fmt.Printf("발견된 객체 수: %d\n\n", len(rbdObjects))

	// 5. 각 객체의 PG 및 OSD 매핑 정보 출력
	fmt.Printf("%-25s %-15s %-20s\n", "Object", "PG", "Acting OSDs")
	fmt.Printf("%s\n", repeat("-", 65))

	// 최대 10개 객체만 표시 (너무 많을 수 있으므로)
	maxObjects := len(rbdObjects)
	if maxObjects > 10 {
		maxObjects = 10
	}

	for i := 0; i < maxObjects; i++ {
		obj := rbdObjects[i]

		// 6. 객체의 PG 매핑 가져오기 (MgrCommand 사용)
		pgid, err := getObjectPGMapping(conn, poolName, obj)
		if err != nil {
			fmt.Printf("%-25s %-15s %-20s\n", obj, "ERROR", err.Error())
			continue
		}

		// 7. PG의 OSD 매핑 가져오기
		osds, err := getPGOSDMapping(conn, pgid)
		if err != nil {
			fmt.Printf("%-25s %-15s %-20s\n", obj, pgid, "ERROR: "+err.Error())
			continue
		}

		osdStr := fmt.Sprintf("%v", osds)
		fmt.Printf("%-25s %-15s %-20s\n", obj, pgid, osdStr)
	}

	if len(rbdObjects) > 10 {
		fmt.Printf("\n... (총 %d개 객체 중 처음 10개만 표시)\n", len(rbdObjects))
	}

	// 8. PG 분포 요약
	fmt.Printf("\n=== PG 분포 요약 ===\n")
	pgCount := make(map[string]int)
	osdSet := make(map[int]bool)

	for i := 0; i < maxObjects; i++ {
		obj := rbdObjects[i]
		pgid, err := getObjectPGMapping(conn, poolName, obj)
		if err != nil {
			continue
		}
		pgCount[pgid]++

		osds, err := getPGOSDMapping(conn, pgid)
		if err != nil {
			continue
		}
		for _, osd := range osds {
			osdSet[osd] = true
		}
	}

	fmt.Printf("사용된 PG 수: %d\n", len(pgCount))
	fmt.Printf("사용된 OSD 수: %d\n", len(osdSet))

	var osdList []int
	for osd := range osdSet {
		osdList = append(osdList, osd)
	}
	fmt.Printf("OSD 목록: %v\n", osdList)
}

// 객체의 PG 매핑 가져오기 (내부 헬퍼 함수)
func getObjectPGMapping(conn *rados.Conn, poolName, objectName string) (string, error) {

	// IOContext를 통한 직접적인 방법 시도
	ioctx, err := conn.OpenIOContext(poolName)
	if err != nil {
		return "", err
	}
	defer ioctx.Destroy()

	// 다른 방법: MonCommand 사용
	cmd, err := json.Marshal(map[string]interface{}{
		"prefix": "osd map",
		"pool":   poolName,
		"object": objectName,
		"format": "json",
	})
	if err != nil {
		return "", err
	}

	// MonCommand로 시도
	buf, _, err := conn.MonCommand(cmd)
	if err != nil {
		// MonCommand도 실패하면 계산으로 PG 찾기
		return calculatePGFromObject(conn, poolName, objectName)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf, &result); err != nil {
		return calculatePGFromObject(conn, poolName, objectName)
	}

	if pgid, ok := result["pgid"].(string); ok {
		return pgid, nil
	}

	// 최후의 수단: 계산으로 PG 찾기
	return calculatePGFromObject(conn, poolName, objectName)
}

// 객체 이름으로부터 PG를 계산하는 함수
func calculatePGFromObject(conn *rados.Conn, poolName, objectName string) (string, error) {
	// Pool 정보 가져오기
	cmd, err := json.Marshal(map[string]interface{}{
		"prefix": "osd pool get",
		"pool":   poolName,
		"var":    "pg_num",
		"format": "json",
	})
	if err != nil {
		return "", err
	}

	buf, _, err := conn.MonCommand(cmd)
	if err != nil {
		return "", fmt.Errorf("pool pg_num 가져오기 실패: %v", err)
	}

	var poolInfo map[string]interface{}
	if err := json.Unmarshal(buf, &poolInfo); err != nil {
		return "", err
	}

	pgNum, ok := poolInfo["pg_num"].(float64)
	if !ok {
		return "", fmt.Errorf("pg_num 파싱 실패")
	}

	// Pool ID 가져오기
	cmd2, err := json.Marshal(map[string]interface{}{
		"prefix": "osd pool ls",
		"detail": true,
		"format": "json",
	})
	if err != nil {
		return "", err
	}

	buf2, _, err := conn.MonCommand(cmd2)
	if err != nil {
		return "", fmt.Errorf("pool 목록 가져오기 실패: %v", err)
	}

	var pools []map[string]interface{}
	if err := json.Unmarshal(buf2, &pools); err != nil {
		return "", err
	}

	var poolID float64 = -1
	for _, pool := range pools {
		if pool["poolname"].(string) == poolName {
			poolID = pool["poolnum"].(float64)
			break
		}
	}

	if poolID == -1 {
		return "", fmt.Errorf("pool ID를 찾을 수 없습니다")
	}

	// 간단한 해시 기반 PG 계산 (실제 Ceph 알고리즘의 단순화된 버전)
	// 실제로는 CRUSH 맵과 복잡한 해시 함수를 사용하지만, 여기서는 근사치 계산
	hash := simpleHash(objectName)
	pgIndex := hash % uint32(pgNum)

	return fmt.Sprintf("%.0f.%x", poolID, pgIndex), nil
}

// 간단한 해시 함수 (실제 Ceph는 더 복잡한 해시 사용)
func simpleHash(s string) uint32 {
	var hash uint32 = 5381
	for _, c := range s {
		hash = ((hash << 5) + hash) + uint32(c)
	}
	return hash
}

// PG의 OSD 매핑 가져오기 (내부 헬퍼 함수)
func getPGOSDMapping(conn *rados.Conn, pgid string) ([]int, error) {
	cmd, err := json.Marshal(map[string]interface{}{
		"prefix": "pg map",
		"pgid":   pgid,
		"format": "json",
	})
	if err != nil {
		return nil, err
	}

	buf, _, err := conn.MonCommand(cmd)
	if err != nil {
		// pg map이 실패하면 pg query로 시도
		return getPGOSDMappingFromQuery(conn, pgid)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf, &result); err != nil {
		return getPGOSDMappingFromQuery(conn, pgid)
	}

	// acting 필드에서 OSD 목록 추출
	if acting, ok := result["acting"].([]interface{}); ok {
		var osds []int
		for _, osd := range acting {
			if osdNum, ok := osd.(float64); ok {
				osds = append(osds, int(osdNum))
			}
		}
		return osds, nil
	}

	// 대안: up 필드 시도
	if up, ok := result["up"].([]interface{}); ok {
		var osds []int
		for _, osd := range up {
			if osdNum, ok := osd.(float64); ok {
				osds = append(osds, int(osdNum))
			}
		}
		return osds, nil
	}

	return getPGOSDMappingFromQuery(conn, pgid)
}

// pg query를 사용한 대안 방법
func getPGOSDMappingFromQuery(conn *rados.Conn, pgid string) ([]int, error) {
	cmd, err := json.Marshal(map[string]interface{}{
		"prefix": "pg query",
		"pgid":   pgid,
		"format": "json",
	})
	if err != nil {
		return nil, err
	}

	buf, _, err := conn.MonCommand(cmd)
	if err != nil {
		return nil, fmt.Errorf("pg query 실패: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf, &result); err != nil {
		return nil, err
	}

	// acting 필드에서 OSD 목록 추출
	if state, ok := result["state"].(map[string]interface{}); ok {
		if acting, ok := state["acting"].([]interface{}); ok {
			var osds []int
			for _, osd := range acting {
				if osdNum, ok := osd.(float64); ok {
					osds = append(osds, int(osdNum))
				}
			}
			return osds, nil
		}
	}

	// info 필드에서 시도
	if info, ok := result["info"].(map[string]interface{}); ok {
		if acting, ok := info["acting"].([]interface{}); ok {
			var osds []int
			for _, osd := range acting {
				if osdNum, ok := osd.(float64); ok {
					osds = append(osds, int(osdNum))
				}
			}
			return osds, nil
		}
	}

	return nil, fmt.Errorf("OSD 목록을 찾을 수 없습니다")
}

// ListRBDImages 풀의 RBD 이미지 목록을 문자열 배열로 반환합니다.
func ListRBDImages(conn *rados.Conn, poolName string) ([]string, error) {
	// GetImages 함수를 호출해 이미지 맵을 가져옵니다
	images, err := GetImages(conn, poolName)
	if err != nil {
		return nil, fmt.Errorf("풀 %s의 이미지 목록 조회 실패: %v", poolName, err)
	}

	// 이미지 이름 목록 추출
	var imageNames []string
	for _, img := range images {
		if name, ok := img["name"].(string); ok {
			imageNames = append(imageNames, name)
		}
	}

	return imageNames, nil
}

func repeat(s string, count int) string {
	var result string
	for i := 0; i < count; i++ {
		result += s
	}
	return result
}

func AnalyzeRBDImageMappingWithDiffIterate(conn *rados.Conn, poolName, imageName string) error {
	slog.Infof("\n=== 🚀 DiffIterate 기반 초고속 매핑 분석 ===")

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

	// 2. 🚀 DiffIterate로 실제 사용된 블록 탐지
	slog.Info("\n=== DiffIterate 기반 사용 블록 탐지 ===")

	var usedObjects []string
	var totalUsedBytes uint64

	config := rbd.DiffIterateConfig{
		Offset:        0,
		Length:        stat.Size,
		SnapName:      rbd.NoSnapshot,
		WholeObject:   rbd.EnableWholeObject,
		IncludeParent: rbd.ExcludeParent,
		Callback: func(offset, length uint64, exists int, data interface{}) int {
			if exists == 1 {
				// 객체 번호 계산
				objNum := offset / uint64(stat.Obj_size)
				objectName := fmt.Sprintf("%s.%016x", stat.Block_name_prefix, objNum)

				// 중복 제거하며 추가
				found := false
				for _, existing := range usedObjects {
					if existing == objectName {
						found = true
						break
					}
				}
				if !found {
					usedObjects = append(usedObjects, objectName)
				}

				totalUsedBytes += length

				// 진행상황 로그 (너무 많으면 생략)
				if len(usedObjects) <= 10 {
					slog.Infof("사용된 블록 발견 - 객체: %s, 오프셋: %d, 길이: %d",
						objectName, offset, length)
				}
			}
			return 0 // 계속 진행
		},
	}

	err = image.DiffIterate(config)
	if err != nil {
		return fmt.Errorf("DiffIterate 실행 실패: %v", err)
	}

	slog.Infof("✅ DiffIterate 완료 - 실제 사용 객체: %d개", len(usedObjects))
	slog.Infof("실제 사용 용량: %.2f MB (%.1f%%)",
		float64(totalUsedBytes)/1024/1024,
		float64(totalUsedBytes)/float64(stat.Size)*100)

	// 3. 사용된 객체들의 PG/OSD 매핑 분석
	pgToOSDs := make(map[string][]int)
	allOSDs := make(map[int]bool)
	pgToObjectCount := make(map[string]int)

	slog.Info("\n=== 사용된 객체들의 매핑 분석 ===")

	for i, objectName := range usedObjects {
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
			slog.Infof("매핑 분석 진행: %d/%d", i+1, len(usedObjects))
		}
	}

	// 4. 최종 결과 출력
	slog.Info("\n=== 🚀 DiffIterate 기반 분석 결과 ===")
	slog.Infof("이미지 총 크기 - %.2f MB", float64(stat.Size)/1024/1024)
	slog.Infof("실제 사용 객체 - %d개", len(usedObjects))
	slog.Infof("실제 사용 용량 - %.2f MB (%.1f%%)",
		float64(totalUsedBytes)/1024/1024,
		float64(totalUsedBytes)/float64(stat.Size)*100)
	slog.Infof("사용된 PG 수 - %d", len(pgToOSDs))
	slog.Infof("사용된 OSD 수 - %d", len(allOSDs))

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
