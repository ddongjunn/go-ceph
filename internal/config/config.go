package config

// Config 애플리케이션 설정
type Config struct {
	ServerAddress  string
	CephConfigPath string
}

// NewConfig 기본 설정으로 설정 객체 생성
func NewConfig() *Config {
	return &Config{
		ServerAddress:  ":9080",
		CephConfigPath: "", // 빈 문자열이면 기본 경로 사용
	}
}
