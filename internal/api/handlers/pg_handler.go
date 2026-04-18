package handlers

import (
	"ceph-core-api/internal/core/pg"
	"ceph-core-api/pkg/models"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// PG 정보 서비스 인스턴스
var pgInfoService = pg.NewPGInfoService()

// GetPGList ceph pg ls 명령어 API 핸들러
func GetPGList(c *gin.Context) {
	result, err := pgInfoService.GetPGList()
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.Response{
			Status:  "error",
			Message: "PG 목록 조회 실패",
			Error:   err.Error(),
		})
		return
	}

	// JSON 문자열을 실제 JSON 객체로 파싱
	var pgData interface{}
	if err := json.Unmarshal([]byte(result), &pgData); err != nil {
		c.JSON(http.StatusInternalServerError, models.Response{
			Status:  "error",
			Message: "JSON 파싱 실패",
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.Response{
		Status:  "success",
		Message: "PG 목록 조회 완료",
		Data: gin.H{
			"pg_list": pgData,
		},
	})
}

// GetPGListByPool ceph pg ls-by-pool <pool-name> 명령어 API 핸들러
func GetPGListByPool(c *gin.Context) {
	poolName := c.Param("pool_name")
	if poolName == "" {
		c.JSON(http.StatusBadRequest, models.Response{
			Status:  "error",
			Message: "풀 이름이 필요합니다",
		})
		return
	}

	result, err := pgInfoService.GetPGListByPool(poolName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.Response{
			Status:  "error",
			Message: "풀별 PG 목록 조회 실패",
			Error:   err.Error(),
		})
		return
	}

	// JSON 문자열을 실제 JSON 객체로 파싱
	var pgData interface{}
	if err := json.Unmarshal([]byte(result), &pgData); err != nil {
		c.JSON(http.StatusInternalServerError, models.Response{
			Status:  "error",
			Message: "JSON 파싱 실패",
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.Response{
		Status:  "success",
		Message: "풀별 PG 목록 조회 완료",
		Data: gin.H{
			"pool_name": poolName,
			"pg_list":   pgData,
		},
	})
}

// GetPGListByPoolID ceph pg ls [<pool:int>] 명령어 API 핸들러
func GetPGListByPoolID(c *gin.Context) {
	poolIDStr := c.Param("pool_id")
	poolID, err := strconv.Atoi(poolIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.Response{
			Status:  "error",
			Message: "올바른 풀 ID가 필요합니다",
			Error:   err.Error(),
		})
		return
	}

	result, err := pgInfoService.GetPGListByPoolID(poolID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.Response{
			Status:  "error",
			Message: "풀 ID별 PG 목록 조회 실패",
			Error:   err.Error(),
		})
		return
	}

	// JSON 문자열을 실제 JSON 객체로 파싱
	var pgData interface{}
	if err := json.Unmarshal([]byte(result), &pgData); err != nil {
		c.JSON(http.StatusInternalServerError, models.Response{
			Status:  "error",
			Message: "JSON 파싱 실패",
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.Response{
		Status:  "success",
		Message: "풀 ID별 PG 목록 조회 완료",
		Data: gin.H{
			"pool_id": poolID,
			"pg_list": pgData,
		},
	})
}

// GetPoolListDetail ceph osd pool ls detail 명령어 API 핸들러
func GetPoolListDetail(c *gin.Context) {
	result, err := pgInfoService.GetPoolListDetail()
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.Response{
			Status:  "error",
			Message: "풀 상세 목록 조회 실패",
			Error:   err.Error(),
		})
		return
	}

	var poolData interface{}
	if err := json.Unmarshal([]byte(result), &poolData); err != nil {
		c.JSON(http.StatusInternalServerError, models.Response{
			Status:  "error",
			Message: "JSON 파싱 실패",
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.Response{
		Status:  "success",
		Message: "풀 상세 목록 조회 완료",
		Data: gin.H{
			"pool_list": poolData,
		},
	})
}

// GetOSDTree ceph osd tree 명령어 API 핸들러
func GetOSDTree(c *gin.Context) {
	result, err := pgInfoService.GetOSDTree()
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.Response{
			Status:  "error",
			Message: "OSD 트리 조회 실패",
			Error:   err.Error(),
		})
		return
	}

	var osdData interface{}
	if err := json.Unmarshal([]byte(result), &osdData); err != nil {
		c.JSON(http.StatusInternalServerError, models.Response{
			Status:  "error",
			Message: "JSON 파싱 실패",
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.Response{
		Status:  "success",
		Message: "OSD 트리 조회 완료",
		Data: gin.H{
			"osd_tree": osdData,
		},
	})
}
