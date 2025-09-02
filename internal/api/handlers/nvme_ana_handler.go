package handlers

import (
	"ceph-core-api/internal/core/nvme"
	"ceph-core-api/pkg/models"
	"net"
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

var (
	anaStateService = nvme.NewANAStateService()
	validate        = validator.New()
)

func init() {
	// IP 주소 validation
	validate.RegisterValidation("ip", validateIP)
	// ANA 상태 validation
	validate.RegisterValidation("ana_state", validateANAState)
	// 전송 프로토콜 validation
	validate.RegisterValidation("transport", validateTransport)
	// 인증 방법 validation
	validate.RegisterValidation("auth_method", validateAuthMethod)
	// 컨테이너 이름 validation
	validate.RegisterValidation("container_name", validateContainerName)
	// NQN validation
	validate.RegisterValidation("nqn", validateNQN)
}

// NVMeANARequest ANA 상태 설정 요청 구조체 (향상된 validation)
type NVMeANARequest struct {
	Host          string `json:"host" binding:"required,min=1,max=255" validate:"required"`
	User          string `json:"user" binding:"required,min=1,max=64" validate:"required"`
	AuthMethod    string `json:"auth_method" binding:"required" validate:"required,auth_method"`
	AuthValue     string `json:"auth_value" binding:"required,min=1" validate:"required"`
	ContainerName string `json:"container_name" binding:"required" validate:"required,container_name"`
	ANAState      string `json:"ana_state" binding:"required" validate:"required,ana_state"`
	Transport     string `json:"transport" binding:"required" validate:"required,transport"`
	Address       string `json:"address" binding:"required" validate:"required,ip"`
	Port          int    `json:"port" binding:"required,min=1,max=65535" validate:"required,min=1,max=65535"`
	NQN           string `json:"nqn" binding:"required" validate:"required,nqn"`
}

// 커스텀 validation 함수들
func validateIP(fl validator.FieldLevel) bool {
	ip := fl.Field().String()
	return net.ParseIP(ip) != nil
}

func validateANAState(fl validator.FieldLevel) bool {
	state := fl.Field().String()
	validStates := []string{"optimized", "non_optimized", "inaccessible", "change"}
	for _, validState := range validStates {
		if state == validState {
			return true
		}
	}
	return false
}

func validateTransport(fl validator.FieldLevel) bool {
	transport := fl.Field().String()
	validTransports := []string{"tcp", "rdma", "fc"}
	for _, validTransport := range validTransports {
		if transport == validTransport {
			return true
		}
	}
	return false
}

func validateAuthMethod(fl validator.FieldLevel) bool {
	method := fl.Field().String()
	return method == "password" || method == "key"
}

func validateContainerName(fl validator.FieldLevel) bool {
	name := fl.Field().String()
	// 컨테이너 이름 패턴 검증 (영문자, 숫자, 점, 하이픈, 언더스코어)
	matched, _ := regexp.MatchString(`^[a-zA-Z0-9._-]+$`, name)
	return matched && len(name) > 0 && len(name) <= 253
}

func validateNQN(fl validator.FieldLevel) bool {
	nqn := fl.Field().String()
	// NQN 형식 검증: nqn.yyyy-mm.reverse-domain:identifier
	matched, _ := regexp.MatchString(`^nqn\.\d{4}-\d{2}\.[^:]+:.+$`, nqn)
	return matched
}

// 상세한 validation 에러 메시지 생성
func getValidationErrorMessage(err error) map[string]string {
	errors := make(map[string]string)

	if validationErrors, ok := err.(validator.ValidationErrors); ok {
		for _, fieldError := range validationErrors {
			field := strings.ToLower(fieldError.Field())

			switch fieldError.Tag() {
			case "required":
				errors[field] = field + " 필드는 필수입니다"
			case "min":
				errors[field] = field + " 필드는 최소 " + fieldError.Param() + "자 이상이어야 합니다"
			case "max":
				errors[field] = field + " 필드는 최대 " + fieldError.Param() + "자 이하여야 합니다"
			case "ip":
				errors[field] = "올바른 IP 주소 형식이 아닙니다"
			case "ana_state":
				errors[field] = "ANA 상태는 optimized, non_optimized, inaccessible, change 중 하나여야 합니다"
			case "transport":
				errors[field] = "전송 프로토콜은 tcp, rdma, fc 중 하나여야 합니다"
			case "auth_method":
				errors[field] = "인증 방법은 password 또는 key여야 합니다"
			case "container_name":
				errors[field] = "올바른 컨테이너 이름 형식이 아닙니다"
			case "nqn":
				errors[field] = "올바른 NQN 형식이 아닙니다 (예: nqn.2001-07.com.example:identifier)"
			default:
				errors[field] = field + " 필드가 유효하지 않습니다"
			}
		}
	}

	return errors
}

// SetNVMeANAStateHandler NVMe ANA 상태 설정 핸들러 (향상된 validation)
func SetNVMeANAStateHandler(c *gin.Context) {
	var req NVMeANARequest

	// JSON 바인딩 및 기본 validation
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.Response{
			Status:  "error",
			Message: "잘못된 요청 형식",
			Error:   err.Error(),
			Data: gin.H{
				"validation_errors": getValidationErrorMessage(err),
			},
		})
		return
	}

	// 추가 커스텀 validation
	if err := validate.Struct(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.Response{
			Status:  "error",
			Message: "요청 데이터 검증 실패",
			Error:   err.Error(),
			Data: gin.H{
				"validation_errors": getValidationErrorMessage(err),
			},
		})
		return
	}

	// 비즈니스 로직 validation
	if err := validateBusinessLogic(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.Response{
			Status:  "error",
			Message: "비즈니스 규칙 검증 실패",
			Error:   err.Error(),
		})
		return
	}

	// 요청을 서비스 구조체로 변환
	config := &nvme.ANAStateConfig{
		ContainerName: req.ContainerName,
		ANAState:      req.ANAState,
		Transport:     req.Transport,
		Address:       req.Address,
		Port:          req.Port,
		NQN:           req.NQN,
	}

	// ANA 상태 서비스 호출
	result, err := anaStateService.SetANAState(req.Host, req.User, req.AuthMethod, req.AuthValue, config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.Response{
			Status:  "error",
			Message: "NVMe ANA 상태 설정 실패",
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.Response{
		Status:  "success",
		Message: "NVMe ANA 상태 설정 완료",
		Data: gin.H{
			"result":    result,
			"ana_state": req.ANAState,
			"container": req.ContainerName,
			"nqn":       req.NQN,
		},
	})
}

// 비즈니스 로직 validation
func validateBusinessLogic(req *NVMeANARequest) error {
	// 포트 범위 추가 검증
	if req.Port < 1024 && req.Port != 80 && req.Port != 443 {
		return validator.ValidationErrors{}
	}

	// 특정 조합 검증
	if req.Transport == "rdma" && req.Port < 4420 {
		return validator.ValidationErrors{}
	}

	return nil
}

// GetNVMeANAStateHandler NVMe 서브시스템 상태 조회 핸들러 (validation 추가)
func GetNVMeANAStateHandler(c *gin.Context) {
	host := c.Query("host")
	user := c.Query("user")
	authMethod := c.Query("auth_method")
	authValue := c.Query("auth_value")
	containerName := c.Query("container_name")

	// 필수 파라미터 validation
	validationErrors := make(map[string]string)

	if host == "" {
		validationErrors["host"] = "host 파라미터는 필수입니다"
	}
	if user == "" {
		validationErrors["user"] = "user 파라미터는 필수입니다"
	}
	if authMethod == "" {
		validationErrors["auth_method"] = "auth_method 파라미터는 필수입니다"
	} else if authMethod != "password" && authMethod != "key" {
		validationErrors["auth_method"] = "auth_method는 password 또는 key여야 합니다"
	}
	if authValue == "" {
		validationErrors["auth_value"] = "auth_value 파라미터는 필수입니다"
	}
	if containerName == "" {
		validationErrors["container_name"] = "container_name 파라미터는 필수입니다"
	}

	if len(validationErrors) > 0 {
		c.JSON(http.StatusBadRequest, models.Response{
			Status:  "error",
			Message: "필수 파라미터 누락 또는 잘못된 값",
			Data: gin.H{
				"validation_errors": validationErrors,
			},
		})
		return
	}
}
