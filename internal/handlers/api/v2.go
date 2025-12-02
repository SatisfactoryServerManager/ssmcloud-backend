package api

import (
	"os"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/handlers/api/frontend"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/utils/config"
	"github.com/gin-gonic/gin"
)

type V2Handler struct{}

func (h *V2Handler) API_Ping(c *gin.Context) {

	hostname, _ := os.Hostname()
	configData, _ := config.GetConfigData()

	c.JSON(200, gin.H{"success": true, "node": hostname, "version": configData.Version})
}

func NewV2Handler(router *gin.RouterGroup) {
	group := router.Group("v2")
	handler := V2Handler{}

	group.GET("/ping", handler.API_Ping)

	FrontendGroup := group.Group("frontend")
	frontend.NewFrontendHandler(*FrontendGroup)
}
