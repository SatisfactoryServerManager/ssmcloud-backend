package handlers

import (
	"fmt"
	"net/http"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/middleware"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/repositories"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/services"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/types"
	models "github.com/SatisfactoryServerManager/ssmcloud-resources/models/v1"
	v2 "github.com/SatisfactoryServerManager/ssmcloud-resources/models/v2"
	"github.com/gin-gonic/gin"
	"github.com/mrhid6/go-mongoose/mongoose"
	"go.mongodb.org/mongo-driver/bson"
)

type ApiAgentHandler struct{}

func (h *ApiAgentHandler) API_UpdateAgentStatus(c *gin.Context) {
	AgentAPIKey := c.GetString("AgentKey")

	var PostData types.API_AgentStatus_PutData
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

func (h *ApiAgentHandler) API_UploadAgentSave(c *gin.Context) {

	AgentAPIKey := c.GetString("AgentKey")
	FileIdentity := c.Keys["FileIdentity"].(types.StorageFileIdentity)

	err := services.UploadedAgentSave(AgentAPIKey, FileIdentity, false)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *ApiAgentHandler) API_UploadAgentBackup(c *gin.Context) {

	AgentAPIKey := c.GetString("AgentKey")
	FileIdentity := c.Keys["FileIdentity"].(types.StorageFileIdentity)

	err := services.UploadedAgentBackup(AgentAPIKey, FileIdentity)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *ApiAgentHandler) API_UploadAgentLog(c *gin.Context) {
	AgentAPIKey := c.GetString("AgentKey")
	FileIdentity := c.Keys["FileIdentity"].(types.StorageFileIdentity)

	err := services.UploadedAgentLog(AgentAPIKey, FileIdentity)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *ApiAgentHandler) API_GetModConfig(c *gin.Context) {
	AgentAPIKey := c.GetString("AgentKey")

	config, err := services.GetAgentModConfig(AgentAPIKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": config})
}

func (h *ApiAgentHandler) API_UpdateModConfig(c *gin.Context) {
	AgentAPIKey := c.GetString("AgentKey")

	var PostData v2.AgentModConfig
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

func (h *ApiAgentHandler) API_GetAgentTasks(c *gin.Context) {
	AgentAPIKey := c.GetString("AgentKey")

	tasks, err := services.GetAgentTasksApi(AgentAPIKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": tasks})
}

func (h *ApiAgentHandler) API_UpdateTaskItem(c *gin.Context) {
	AgentAPIKey := c.GetString("AgentKey")
	TaskID := c.Param("taskid")

	var PostData types.API_AgentTaskItem_PutData
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

func (h *ApiAgentHandler) API_GetAgentConfig(c *gin.Context) {
	AgentAPIKey := c.GetString("AgentKey")

	config, err := services.GetAgentConfig(AgentAPIKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": config})
}

func (h *ApiAgentHandler) API_UpdateAgentConfig(c *gin.Context) {
	AgentAPIKey := c.GetString("AgentKey")

	var PostData types.API_AgentConfig_PutData
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

func (h *ApiAgentHandler) API_DownloadAgentSave(c *gin.Context) {
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

	var theSave v2.AgentSave
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

	objectPath := fmt.Sprintf("%s/%s/saves/%s", theAccount.ID.Hex(), theAgent.ID.Hex(), theSave.FileName)

	object, err := repositories.GetAgentFile(objectPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	defer object.Close()

	objectInfo, err := object.Stat()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	// Set headers to force file download
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", theSave.FileName))
	c.Header("Content-Type", objectInfo.ContentType)
	c.Header("Content-Length", fmt.Sprintf("%d", objectInfo.Size))

	// Stream the file to the response
	c.DataFromReader(http.StatusOK, objectInfo.Size, objectInfo.ContentType, object, nil)
}

func (h *ApiAgentHandler) API_GetSyncSaves(c *gin.Context) {
	AgentAPIKey := c.GetString("AgentKey")

	saves, err := services.GetAgentSaves(AgentAPIKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"saves": saves}})
}

func (h *ApiAgentHandler) API_PostSyncSaves(c *gin.Context) {
	AgentAPIKey := c.GetString("AgentKey")

	type postdata struct {
		Saves []v2.AgentSave `json:"saves"`
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

	handler := ApiAgentHandler{}

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
