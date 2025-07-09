package handlers

import (
	"ceph-core-api/internal/core/rados"
	"ceph-core-api/pkg/models"
	"github.com/gin-gonic/gin"
	"net/http"
)

// GetClusterFSID Ceph 클러스터 FSID 조회 핸들러
func GetClusterFSID(c *gin.Context) {
	conn, err := rados.GetConnection()
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.Response{
			Status:  "error",
			Message: "Ceph 클라이언트 생성 실패",
			Error:   err.Error(),
		})
		return
	}

	fsid, err := conn.GetFSID()
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

// GetClusterStatus Ceph 클러스터 상태 조회 핸들러
//func GetClusterStatus(c *gin.Context) {
//	client, err := rados.NewClient("")
//	if err != nil {
//		c.JSON(http.StatusInternalServerError, models.Response{
//			Status:  "error",
//			Message: "Ceph 클라이언트 생성 실패",
//			Error:   err.Error(),
//		})
//		return
//	}
//	defer client.Close()
//
//	status, err := client.GetClusterStatus()
//	if err != nil {
//		c.JSON(http.StatusInternalServerError, models.Response{
//			Status:  "error",
//			Message: "클러스터 상태 조회 실패",
//			Error:   err.Error(),
//		})
//		return
//	}
//
//	c.JSON(http.StatusOK, models.Response{
//		Status: "success",
//		Data:   status,
//	})
//}
