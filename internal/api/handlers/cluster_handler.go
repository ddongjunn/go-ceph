package handlers

import (
	"ceph-core-api/internal/core/cluster"
	"ceph-core-api/pkg/models"
	"net/http"

	"github.com/gin-gonic/gin"
)

// 클러스터 서비스 인스턴스 (전역 또는 의존성 주입)
var clusterService = cluster.NewInfoService()

// GetClusterFSID Ceph 클러스터 FSID 조회 핸들러
func GetClusterFSID(c *gin.Context) {
	// 서비스 계층 호출
	fsid, err := clusterService.GetFSID()
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.Response{
			Status:  "error",
			Message: "클러스터 FSID 조회 실패",
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.Response{
		Status: "success",
		Data: gin.H{
			"fsid": fsid,
		},
	})
}
