package handlers

import (
	"net/http"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/app"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/middleware"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/services"
	"github.com/gin-gonic/gin"
)

type SSMModHandler struct{}

func (h *SSMModHandler) API_UpdatePlayers(c *gin.Context) {
	AgentAPIKey := c.GetString("AgentKey")

	var PostData app.API_UpdatePlayers_PostData
	if err := c.BindJSON(&PostData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	err := services.UpdateAgentPlayers(AgentAPIKey, PostData)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *SSMModHandler) API_UpdateBuildings(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func NewSSMModHandler(router *gin.RouterGroup) {
	handler := SSMModHandler{}

	router.Use(middleware.Middleware_AgentAPIKey())

	router.POST("/players", handler.API_UpdatePlayers)
	router.POST("/buildings", handler.API_UpdateBuildings)
}
