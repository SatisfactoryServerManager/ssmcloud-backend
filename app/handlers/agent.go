package handlers

import (
	"fmt"
	"net/http"
	"path/filepath"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/app"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/middleware"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/models"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/services"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/utils/config"
	"github.com/gin-gonic/gin"
	"github.com/mrhid6/go-mongoose/mongoose"
	"go.mongodb.org/mongo-driver/bson"
)

type AgentHandler struct{}

func (h *AgentHandler) API_UpdateAgentStatus(c *gin.Context) {
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

func (h *AgentHandler) API_UploadAgentSave(c *gin.Context) {

	AgentAPIKey := c.GetString("AgentKey")
	FileIdentity := c.Keys["FileIdentity"].(services.StorageFileIdentity)

	err := services.UploadedAgentSave(AgentAPIKey, FileIdentity, false)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *AgentHandler) API_UploadAgentBackup(c *gin.Context) {

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

func (h *AgentHandler) API_UploadAgentLog(c *gin.Context) {
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

func (h *AgentHandler) API_GetModConfig(c *gin.Context) {
	AgentAPIKey := c.GetString("AgentKey")

	config, err := services.GetAgentModConfig(AgentAPIKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": config})
}

func (h *AgentHandler) API_UpdateModConfig(c *gin.Context) {
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

func (h *AgentHandler) API_GetAgentTasks(c *gin.Context) {
	AgentAPIKey := c.GetString("AgentKey")

	tasks, err := services.GetAgentTasksApi(AgentAPIKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": tasks})
}

func (h *AgentHandler) API_UpdateTaskItem(c *gin.Context) {
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

func (h *AgentHandler) API_GetAgentConfig(c *gin.Context) {
	AgentAPIKey := c.GetString("AgentKey")

	config, err := services.GetAgentConfig(AgentAPIKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": config})
}

func (h *AgentHandler) API_UpdateAgentConfig(c *gin.Context) {
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

func (h *AgentHandler) API_DownloadAgentSave(c *gin.Context) {
	AgentAPIKey := c.GetString("AgentKey")
	SaveFileName := c.Param("filename")

	theAgent, err := services.GetAgentByAPIKey(AgentAPIKey)
	if err != nil {
		err = fmt.Errorf("error finding agent with error: %s", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	var theAccount models.Accounts
	if err := mongoose.FindOne(bson.M{"agents": theAgent.ID}, &theAccount); err != nil {
		err = fmt.Errorf("error finding agent account with error: %s", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	var theSave models.AgentSave
	for _, save := range theAgent.Saves {
		if save.FileName == SaveFileName {
			theSave = save
			break
		}
	}

	if theSave.FileName == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "save file not found", "success": false})
		c.Abort()
		return
	}

	newFilePath := filepath.Join(config.DataDir, "account_data", theAccount.ID.Hex(), theAgent.ID.Hex(), "saves")
	newFileLocation := filepath.Join(newFilePath, theSave.FileName)

	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Content-Disposition", "attachment; filename="+theSave.FileName)
	c.Header("Content-Type", "application/octet-stream")
	c.File(newFileLocation)
}

func (h *AgentHandler) API_GetSyncSaves(c *gin.Context) {
	AgentAPIKey := c.GetString("AgentKey")

	saves, err := services.GetAgentSaves(AgentAPIKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"saves": saves}})
}

func (h *AgentHandler) API_PostSyncSaves(c *gin.Context) {
	AgentAPIKey := c.GetString("AgentKey")

	type postdata struct {
		Saves []models.AgentSave `json:"saves"`
	}

	var PostData postdata
	if err := c.BindJSON(&PostData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	err := services.PostAgentSyncSaves(AgentAPIKey, PostData.Saves)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func NewAgentHandler(router *gin.RouterGroup) {

	handler := AgentHandler{}

	router.Use(middleware.Middleware_AgentAPIKey())

	router.PUT("/status", handler.API_UpdateAgentStatus)

	router.GET("/modconfig", handler.API_GetModConfig)
	router.PUT("/modconfig", handler.API_UpdateModConfig)

	router.GET("/tasks", handler.API_GetAgentTasks)
	router.PUT("/tasks/:taskid", handler.API_UpdateTaskItem)

	router.GET("/config", handler.API_GetAgentConfig)
	router.PUT("/config", handler.API_UpdateAgentConfig)

	router.GET("/save/sync", handler.API_GetSyncSaves)
	router.POST("/save/sync", handler.API_PostSyncSaves)

	uploadGroup := router.Group("upload")
	uploadGroup.Use(middleware.Middleware_UploadFile())

	uploadGroup.POST("/save", handler.API_UploadAgentSave)
	uploadGroup.POST("/backup", handler.API_UploadAgentBackup)
	uploadGroup.POST("/log", handler.API_UploadAgentLog)

	router.GET("/saves/download/:filename", handler.API_DownloadAgentSave)
}
