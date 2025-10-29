package handlers

import (
	"net/http"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/services"
	"github.com/gin-gonic/gin"
)

type AccountAgentHandler struct{}

func (h *AccountAgentHandler) API_GetAgentMapData(c *gin.Context) {
	AgentID := c.Param("agentid")

	agent, err := services.GetAgentByIdNoAccount(AgentID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": agent.MapData})
}

func NewAccountAgentHandler(router *gin.RouterGroup) {
	handler := AccountAgentHandler{}

	router.GET("/:agentid/mapdata", handler.API_GetAgentMapData)
}
