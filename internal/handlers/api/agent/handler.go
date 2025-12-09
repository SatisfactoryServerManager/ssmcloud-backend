package agent

import (
	"fmt"
	"net/http"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/middleware"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/repositories"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/services/agent"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/types"
	v2 "github.com/SatisfactoryServerManager/ssmcloud-resources/models/v2"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
)

type LogUpdate struct {
	Source    string `json:"source"`
	Line      string `json:"line"`
	Timestamp int64  `json:"timestamp"`
}

type ApiAgentHandler struct{}

func (h *ApiAgentHandler) API_UploadAgentSave(c *gin.Context) {

	AgentAPIKey := c.GetString("AgentKey")
	FileIdentity := c.Keys["FileIdentity"].(types.StorageFileIdentity)

	err := agent.UploadedAgentSave(AgentAPIKey, FileIdentity, false)
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

	err := agent.UploadedAgentBackup(AgentAPIKey, FileIdentity)
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

	err := agent.UploadedAgentLog(AgentAPIKey, FileIdentity)
	if err != nil {
		fmt.Printf("%+v\n", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *ApiAgentHandler) API_DownloadAgentSave(c *gin.Context) {
	AgentAPIKey := c.GetString("AgentKey")
	SaveFileName := c.Param("filename")

	// --- Find account ---
	AccountModel, err := repositories.GetMongoClient().GetModel("Account")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		return
	}

	theAgent, err := agent.GetAgentByAPIKey(AgentAPIKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   fmt.Sprintf("error finding agent: %s", err),
			"success": false,
		})
		return
	}

	theAccount := &v2.AccountSchema{}
	if err := AccountModel.FindOne(theAccount, bson.M{"agents": theAgent.ID}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   fmt.Sprintf("error finding agent account: %s", err),
			"success": false,
		})
		return
	}

	// --- Find save file ---
	var theSave *v2.AgentSave
	for idx := range theAgent.Saves {
		save := &theAgent.Saves[idx]
		if save.FileName == SaveFileName {
			theSave = save
			break
		}
	}

	if theSave == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "save file not found", "success": false})
		return
	}

	objectPath := fmt.Sprintf("%s/%s/saves/%s",
		theAccount.ID.Hex(),
		theAgent.ID.Hex(),
		theSave.FileName,
	)

	// --- Get object from S3 ---
	obj, err := repositories.GetAgentFile(objectPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		return
	}
	defer obj.Body.Close()

	// --- Extract metadata ---
	size := obj.ContentLength
	contentType := "application/octet-stream"
	if obj.ContentType != nil {
		contentType = *obj.ContentType
	}

	// --- Prepare download headers ---
	c.Header("Content-Disposition",
		fmt.Sprintf(`attachment; filename="%s"`, theSave.FileName),
	)
	c.Header("Content-Type", contentType)
	c.Header("Content-Length", fmt.Sprintf("%d", size))

	// --- Stream object body to client ---
	c.DataFromReader(http.StatusOK, *size, contentType, obj.Body, nil)
}

func (h *ApiAgentHandler) API_GetSyncSaves(c *gin.Context) {
	AgentAPIKey := c.GetString("AgentKey")

	saves, err := agent.GetAgentSaves(AgentAPIKey)
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

	err := agent.PostAgentSyncSaves(AgentAPIKey, PostData.Saves)
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

	router.GET("/save/sync", handler.API_GetSyncSaves)
	router.POST("/save/sync", handler.API_PostSyncSaves)

	uploadGroup := router.Group("upload")
	uploadGroup.Use(middleware.Middleware_UploadFile())

	uploadGroup.POST("/save", handler.API_UploadAgentSave)
	uploadGroup.POST("/backup", handler.API_UploadAgentBackup)
	uploadGroup.POST("/log", handler.API_UploadAgentLog)

	router.GET("/saves/download/:filename", handler.API_DownloadAgentSave)
}
