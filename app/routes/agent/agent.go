package agent

import (
	"net/http"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/app"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/services"
	"github.com/gin-gonic/gin"
)

func API_UpdateAgentStatus(c *gin.Context) {
	AgentAPIKey := c.GetString("AgentKey")

	var PostData app.API_AgentStatus_PutData
	if err := c.BindJSON(&PostData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	err := services.UpdateAgentStatus(AgentAPIKey, PostData.Online, PostData.Installed, PostData.Running, PostData.CPU, PostData.MEM)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func API_UploadAgentSave(c *gin.Context) {

	AgentAPIKey := c.GetString("AgentKey")
	FileIdentity := c.Keys["FileIdentity"].(services.StorageFileIdentity)

	err := services.UploadedAgentSave(AgentAPIKey, FileIdentity)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func API_UploadAgentBackup(c *gin.Context) {

	AgentAPIKey := c.GetString("AgentKey")
	FileIdentity := c.Keys["FileIdentity"].(services.StorageFileIdentity)

	err := services.UploadedAgentBackup(AgentAPIKey, FileIdentity)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func API_UploadAgentLog(c *gin.Context) {
	AgentAPIKey := c.GetString("AgentKey")
	FileIdentity := c.Keys["FileIdentity"].(services.StorageFileIdentity)

	err := services.UploadedAgentLog(AgentAPIKey, FileIdentity)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func API_GetModConfig(c *gin.Context) {
	AgentAPIKey := c.GetString("AgentKey")

	config, err := services.GetAgentModConfig(AgentAPIKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "config": config})
}
