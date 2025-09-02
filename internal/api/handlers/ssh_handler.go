package handlers

import (
	"ceph-core-api/internal/core/ssh"
	"net/http"

	"github.com/gin-gonic/gin"
)

type SSHCommandRequest struct {
	Host       string `json:"host" binding:"required"`
	User       string `json:"user" binding:"required"`
	AuthMethod string `json:"auth_method" binding:"required,oneof=password key"`
	AuthValue  string `json:"auth_value" binding:"required"`
	Command    string `json:"command" binding:"required"`
	Timeout    int    `json:"timeout,omitempty"`
}

type SSHCommandResponse struct {
	Output string `json:"output"`
	Status int    `json:"status"`
}

func ExecuteSSHCommandHandler(c *gin.Context) {
	var req SSHCommandRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	output, err := ssh.ExecuteSSHCommand(
		req.Host, req.User, req.AuthMethod, req.AuthValue, req.Command, req.Timeout,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, SSHCommandResponse{
		Output: output,
		Status: 0,
	})
}
