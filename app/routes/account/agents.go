package account

import (
	"net/http"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/app"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/services"
	"github.com/gin-gonic/gin"
)

func API_GetAllAgents(c *gin.Context) {
	JWTData, _ := c.Keys["SessionJWT"].(app.Middleware_Session_JWT)
	AccountID := JWTData.AccountID

	agents, err := services.GetAllAgents(AccountID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "agents": agents})
}

func API_CreateNewAgent(c *gin.Context) {

	JWTData, _ := c.Keys["SessionJWT"].(app.Middleware_Session_JWT)
	AccountID := JWTData.AccountID

	var PostData app.API_AccountCreateAgent_PostData
	if err := c.BindJSON(&PostData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	if err := services.CreateAgent(AccountID, PostData.AgentName, PostData.Port, PostData.Memory); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}
