package api

import (
	"os"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/handlers/api/ssmmod"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/utils/config"
	"github.com/gin-gonic/gin"
)

type V1Handler struct{}

func (h *V1Handler) API_Ping(c *gin.Context) {

	hostname, _ := os.Hostname()
	configData, _ := config.GetConfigData()

	c.JSON(200, gin.H{"success": true, "node": hostname, "version": configData.Version})
}

func NewV1Handler(router *gin.RouterGroup) {
	group := router.Group("v1")
	handler := V1Handler{}

	SSMModGroup := group.Group("ssmmod")

	group.GET("/ping", handler.API_Ping)

	ssmmod.NewSSMModHandler(SSMModGroup)
}
