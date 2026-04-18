package handlers

import (
	"ceph-core-api/internal/core/cephfs"
	"net/http"

	"github.com/gin-gonic/gin"
)

// CephFSRequest CephFS 관리 요청 구조체
type CephFSRequest struct {
	FSName string `json:"fs_name" binding:"required"`
}

// CephFSResponse CephFS 관리 응답 구조체
type CephFSResponse struct {
	Message string `json:"message"`
	FSName  string `json:"fs_name"`
	Status  string `json:"status"`
}

var cephfsEditService = cephfs.NewEditService()

func DisableCephFSHandler(c *gin.Context) {
	var req CephFSRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 서비스 계층 호출
	err := cephfsEditService.DisableForEdit(req.FSName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "CephFS 편집 모드 활성화 실패",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, CephFSResponse{
		Message: "CephFS 편집 모드가 성공적으로 활성화되었습니다",
		FSName:  req.FSName,
		Status:  "disabled_for_edit",
	})
}

// EnableCephFSHandler CephFS 편집 후 활성화 API 핸들러
func EnableCephFSHandler(c *gin.Context) {
	var req CephFSRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 서비스 계층 호출
	err := cephfsEditService.EnableAfterEdit(req.FSName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "CephFS 편집 후 활성화 실패",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, CephFSResponse{
		Message: "CephFS가 성공적으로 활성화되었습니다",
		FSName:  req.FSName,
		Status:  "enabled",
	})
}
