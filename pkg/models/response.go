package models

// Response API 응답 표준 구조체
type Response struct {
	Status  string      `json:"status"`            // "success" 또는 "error"
	Message string      `json:"message,omitempty"` // 응답 메시지
	Error   string      `json:"error,omitempty"`   // 에러 메시지 (있는 경우)
	Data    interface{} `json:"data,omitempty"`    // 응답 데이터
}
