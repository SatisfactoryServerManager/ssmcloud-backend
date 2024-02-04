package agent

import (
	"net/http"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/app"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/models"
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

	err := services.UpdateAgentStatus(AgentAPIKey, PostData.Online, PostData.Installed, PostData.Running, PostData.CPU, PostData.MEM, PostData.InstalledSFVersion, PostData.LatestSFVersion)
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

	c.JSON(http.StatusOK, gin.H{"success": true, "data": config})
}

func API_UpdateModConfig(c *gin.Context) {
	AgentAPIKey := c.GetString("AgentKey")

	var PostData models.AgentModConfig
	if err := c.BindJSON(&PostData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	err := services.UpdateAgentModConfig(AgentAPIKey, PostData)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func API_GetAgentTasks(c *gin.Context) {
	AgentAPIKey := c.GetString("AgentKey")

	tasks, err := services.GetAgentTasksApi(AgentAPIKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": tasks})
}

func API_UpdateTaskItem(c *gin.Context) {
	AgentAPIKey := c.GetString("AgentKey")
	TaskID := c.Param("taskid")

	var PostData app.API_AgentTaskItem_PutData
	if err := c.BindJSON(&PostData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	err := services.UpdateAgentTaskItem(AgentAPIKey, TaskID, PostData.Item)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func API_GetAgentConfig(c *gin.Context) {
	AgentAPIKey := c.GetString("AgentKey")

	config, err := services.GetAgentConfig(AgentAPIKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": config})
}

func API_UpdateAgentConfig(c *gin.Context) {
	AgentAPIKey := c.GetString("AgentKey")

	var PostData app.API_AgentConfig_PutData
	if err := c.BindJSON(&PostData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	err := services.UpdateAgentConfigApi(AgentAPIKey, PostData.Version, PostData.IP)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}
