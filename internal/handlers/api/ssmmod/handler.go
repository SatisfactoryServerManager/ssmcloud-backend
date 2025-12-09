package ssmmod

import (
	"net/http"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/middleware"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/services/ssmmod"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/types"
	"github.com/gin-gonic/gin"
)

type ApiSSMModHandler struct{}

func (h *ApiSSMModHandler) API_UpdatePlayers(c *gin.Context) {
	AgentAPIKey := c.GetString("AgentKey")

	var PostData types.API_UpdatePlayers_PostData
	if err := c.BindJSON(&PostData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	err := ssmmod.UpdateAgentPlayers(AgentAPIKey, PostData)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *ApiSSMModHandler) API_UpdateBuildings(c *gin.Context) {
	AgentAPIKey := c.GetString("AgentKey")

	var PostData types.API_UpdateBuildings_PostData
	if err := c.BindJSON(&PostData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	err := ssmmod.UpdateAgentBuildings(AgentAPIKey, PostData)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func NewSSMModHandler(router *gin.RouterGroup) {
	handler := ApiSSMModHandler{}

	router.Use(middleware.Middleware_AgentAPIKey())

	router.POST("/players", handler.API_UpdatePlayers)
	router.POST("/buildings", handler.API_UpdateBuildings)
}
