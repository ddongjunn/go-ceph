package ssh

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"time"

	"ceph-core-api/internal/logger"

	"golang.org/x/crypto/ssh"
)

// Config SSH 클라이언트 설정
type Config struct {
	Host        string
	User        string
	Password    string
	KeyPath     string
	Port        int
	Timeout     time.Duration
	KnownHosts  string
	InsecureSSH bool
}

// Client SSH 클라이언트 구조체
type Client struct {
	config *Config
	client *ssh.Client
}

// NewConfig 기본 SSH 설정 생성
func NewConfig() *Config {
	return &Config{
		Port:    22,
		Timeout: 10 * time.Second,
	}
}

// NewClient SSH 클라이언트 생성
func NewClient(config *Config) *Client {
	return &Client{
		config: config,
	}
}

// WithPassword 비밀번호 인증 설정
func (c *Config) WithPassword(password string) *Config {
	c.Password = password
	return c
}

// WithKeyFile 키 파일 인증 설정
func (c *Config) WithKeyFile(keyPath string) *Config {
	c.KeyPath = keyPath
	return c
}

// WithTimeout 타임아웃 설정
func (c *Config) WithTimeout(timeout time.Duration) *Config {
	c.Timeout = timeout
	return c
}

// WithInsecureSSH 호스트 키 검증 비활성화 (개발용, 프로덕션에서는 사용하지 말 것)
func (c *Config) WithInsecureSSH() *Config {
	c.InsecureSSH = true
	return c
}

// Connect SSH 서버에 연결
func (c *Client) Connect() error {
	// SSH 설정 구성
	config := &ssh.ClientConfig{
		User:    c.config.User,
		Timeout: c.config.Timeout,
	}

	// 인증 방식 설정
	if c.config.Password != "" {
		config.Auth = []ssh.AuthMethod{
			ssh.Password(c.config.Password),
		}
	} else if c.config.KeyPath != "" {
		key, err := ioutil.ReadFile(c.config.KeyPath)
		if err != nil {
			return fmt.Errorf("SSH 키 파일 읽기 실패: %v", err)
		}

		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return fmt.Errorf("SSH 키 파싱 실패: %v", err)
		}

		config.Auth = []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		}
	} else {
		return fmt.Errorf("인증 방식이 지정되지 않았습니다")
	}

	// 호스트 키 검증 설정
	if c.config.InsecureSSH {
		config.HostKeyCallback = ssh.InsecureIgnoreHostKey()
	} else {
		// TODO: 알려진 호스트 키 검증 구현
		config.HostKeyCallback = ssh.InsecureIgnoreHostKey()
	}

	// SSH 연결
	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", c.config.Host, c.config.Port), config)
	if err != nil {
		return fmt.Errorf("SSH 연결 실패: %v", err)
	}

	c.client = client
	return nil
}

// Close SSH 연결 종료
func (c *Client) Close() error {
	if c.client != nil {
		return c.client.Close()
	}
	return nil
}

// ExecuteCommand SSH를 통해 명령어 실행
func (c *Client) ExecuteCommand(command string) (string, error) {
	if c.client == nil {
		if err := c.Connect(); err != nil {
			return "", fmt.Errorf("SSH 연결 실패: %v", err)
		}
		defer c.Close()
	}

	// 세션 생성
	session, err := c.client.NewSession()
	if err != nil {
		return "", fmt.Errorf("SSH 세션 생성 실패: %v", err)
	}
	defer session.Close()

	// 명령어 실행 및 결과 수집
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	logger.Debugf("SSH 명령어 실행: %s", command)
	err = session.Run(command)
	if err != nil {
		return stderr.String(), fmt.Errorf("명령어 실행 실패: %v, 오류: %s", err, stderr.String())
	}

	return stdout.String(), nil
}

// ExecuteCommandWithTimeout 타임아웃이 있는 명령어 실행
func (c *Client) ExecuteCommandWithTimeout(command string, timeout time.Duration) (string, error) {
	resultCh := make(chan struct {
		output string
		err    error
	})

	go func() {
		output, err := c.ExecuteCommand(command)
		resultCh <- struct {
			output string
			err    error
		}{output, err}
	}()

	select {
	case result := <-resultCh:
		return result.output, result.err
	case <-time.After(timeout):
		return "", fmt.Errorf("명령어 실행 타임아웃: %s", command)
	}
}

const DefaultTimeoutSec = 20

func ExecuteSSHCommand(host, user, authMethod, authValue, command string, timeoutSec int) (string, error) {
	config := NewConfig()
	config.Host = host
	config.User = user
	config.WithTimeout(30 * time.Second)
	if authMethod == "password" {
		config.WithPassword(authValue)
	} else if authMethod == "key" {
		config.WithKeyFile(authValue)
	} else {
		return "", fmt.Errorf("지원되지 않는 인증 방식")
	}
	client := NewClient(config)
	defer client.Close()
	if timeoutSec <= 0 {
		timeoutSec = DefaultTimeoutSec
	}
	return client.ExecuteCommandWithTimeout(command, time.Duration(timeoutSec)*time.Second)
}
