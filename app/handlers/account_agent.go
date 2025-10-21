package handlers

import (
	"net/http"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/app"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/middleware"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/services"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/types"
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


func (h *AccountAgentHandler) API_UploadAgentSave(c *gin.Context) {

	JWTData, _ := c.Keys["SessionJWT"].(app.Middleware_Session_JWT)
	AccountID := JWTData.AccountID
	AgentID := c.Param("agentid")

	FileIdentity := c.Keys["FileIdentity"].(types.StorageFileIdentity)

	if _, err := services.GetAccount(AccountID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	theAgent, err := services.GetAgentById(AccountID, AgentID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	if err := services.UploadedAgentSave(theAgent.APIKey, FileIdentity, true); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func NewAccountAgentHandler(router *gin.RouterGroup) {
	handler := AccountAgentHandler{}

	router.GET("/:agentid/mapdata", handler.API_GetAgentMapData)

	router.Use(middleware.Middleware_DecodeJWT())
	router.Use(middleware.Middleware_VerifySession())

	uploadGroup := router.Group("upload")
	uploadGroup.Use(middleware.Middleware_UploadFile())

	uploadGroup.POST("/:agentid/save", handler.API_UploadAgentSave)
}
