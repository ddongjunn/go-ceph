package device

import (
	"ceph-core-api/internal/logger"
	"encoding/json"
	"os/exec"
)

func GetDevices() ([]Host, error) {
	cmd := exec.Command("ceph", "orch", "device", "ls", "-f", "json")

	out, err := cmd.Output()
	if err != nil {
		logger.Errorf("ceph orch device ls 실행 실패: %v", err)
		return nil, err
	}

	var hosts []Host
	if err := json.Unmarshal(out, &hosts); err != nil {
		logger.Errorf("JSON 파싱 실패: %v", err)
		return nil, err
	}

	return hosts, nil
}
