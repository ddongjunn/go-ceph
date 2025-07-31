package device

/*
ceph orch device ls -f json 결과 구조체 정의
*/
type SysAPI struct {
	Model             string  `json:"model"`
	HumanReadableSize string  `json:"human_readable_size"`
	Size              float64 `json:"size"`
}

type Device struct {
	Path              string `json:"path"`
	DeviceID          string `json:"device_id"`
	HumanReadableType string `json:"human_readable_type"`
	Available         bool   `json:"available"`
	SysAPI            SysAPI `json:"sys_api"`
}

type Host struct {
	Name    string   `json:"name"`
	Devices []Device `json:"devices"`
}
