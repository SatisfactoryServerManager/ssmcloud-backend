package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/app"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/middleware"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/models"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/repositories"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/services"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/types"
	"github.com/gin-gonic/gin"
)

type AccountAgentHandler struct{}

func (h *AccountAgentHandler) API_GetAllAgents(c *gin.Context) {
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

func (h *AccountAgentHandler) API_CreateNewAgent(c *gin.Context) {

	JWTData, _ := c.Keys["SessionJWT"].(app.Middleware_Session_JWT)
	AccountID := JWTData.AccountID

	var PostData models.API_AccountCreateAgent_PostData
	if err := c.BindJSON(&PostData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	if PostData.AgentName == "" || PostData.APIKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "required fields are empty", "success": false})
		c.Abort()
		return
	}

	workflowId, err := services.CreateAgentWorkflow(AccountID, PostData)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "workflow_id": workflowId})
}

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

func (h *AccountAgentHandler) API_GetAgentByID(c *gin.Context) {
	JWTData, _ := c.Keys["SessionJWT"].(app.Middleware_Session_JWT)
	AccountID := JWTData.AccountID

	AgentID := c.Param("agentid")

	agent, err := services.GetAgentById(AccountID, AgentID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	Populate := strings.Split(c.Query("populate"), ",")

	if err := agent.PopulateFromURLQuery(Populate); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "agent": agent})
}

func (h *AccountAgentHandler) API_GetAgentTasks(c *gin.Context) {
	JWTData, _ := c.Keys["SessionJWT"].(app.Middleware_Session_JWT)
	AccountID := JWTData.AccountID

	AgentID := c.Param("agentid")

	tasks, err := services.GetAgentTasks(AccountID, AgentID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "tasks": tasks})
}

func (h *AccountAgentHandler) API_NewAgentTask(c *gin.Context) {

	JWTData, _ := c.Keys["SessionJWT"].(app.Middleware_Session_JWT)
	AccountID := JWTData.AccountID

	AgentID := c.Param("agentid")

	var PostData app.API_AccountAgentTask_PostData
	if err := c.BindJSON(&PostData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	err := services.NewAgentTask(AccountID, AgentID, PostData.Action, PostData.Data)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *AccountAgentHandler) API_DeleteAgent(c *gin.Context) {

	JWTData, _ := c.Keys["SessionJWT"].(app.Middleware_Session_JWT)
	AccountID := JWTData.AccountID

	AgentID := c.Param("agentid")

	err := services.DeleteAgent(AccountID, AgentID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *AccountAgentHandler) API_AgentInstallMod(c *gin.Context) {

	JWTData, _ := c.Keys["SessionJWT"].(app.Middleware_Session_JWT)
	AccountID := JWTData.AccountID
	AgentID := c.Param("agentid")

	var PostData app.API_AccountAgentMod_PostData
	if err := c.BindJSON(&PostData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	err := services.UpdateMod(AccountID, AgentID, PostData.ModReference)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *AccountAgentHandler) API_AgentUpdateMod(c *gin.Context) {

	JWTData, _ := c.Keys["SessionJWT"].(app.Middleware_Session_JWT)
	AccountID := JWTData.AccountID
	AgentID := c.Param("agentid")

	var PostData app.API_AccountAgentMod_PostData
	if err := c.BindJSON(&PostData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	err := services.UpdateMod(AccountID, AgentID, PostData.ModReference)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *AccountAgentHandler) API_AgentUninstallMod(c *gin.Context) {

	JWTData, _ := c.Keys["SessionJWT"].(app.Middleware_Session_JWT)
	AccountID := JWTData.AccountID
	AgentID := c.Param("agentid")

	var PostData app.API_AccountAgentMod_PostData
	if err := c.BindJSON(&PostData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	err := services.UninstallMod(AccountID, AgentID, PostData.ModReference)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *AccountAgentHandler) API_GetAgentLogs(c *gin.Context) {
	JWTData, _ := c.Keys["SessionJWT"].(app.Middleware_Session_JWT)
	AccountID := JWTData.AccountID
	AgentID := c.Param("agentid")

	logs, err := services.GetAgentLogs(AccountID, AgentID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "logs": logs})
}

func (h *AccountAgentHandler) API_UpdateAgentConfigs(c *gin.Context) {
	JWTData, _ := c.Keys["SessionJWT"].(app.Middleware_Session_JWT)
	AccountID := JWTData.AccountID
	AgentID := c.Param("agentid")

	var PostData app.API_AccountAgentConfig_PutData
	if err := c.BindJSON(&PostData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	err := services.UpdateAgentConfig(AccountID, AgentID, PostData.UpdatedAgent)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *AccountAgentHandler) API_DownloadAgentBackup(c *gin.Context) {
	JWTData, _ := c.Keys["SessionJWT"].(app.Middleware_Session_JWT)
	AccountID := JWTData.AccountID
	AgentID := c.Param("agentid")
	BackupUUID := c.Param("uuid")

	theAccount, err := services.GetAccount(AccountID)
	if err != nil {
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

	var theBackup models.AgentBackup
	for _, backup := range theAgent.Backups {
		if backup.UUID == BackupUUID {
			theBackup = backup
			break
		}
	}

	if theBackup.FileName == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "backup file not found", "success": false})
		c.Abort()
		return
	}

	objectPath := fmt.Sprintf("%s/%s/backups/%s", theAccount.ID.Hex(), theAgent.ID.Hex(), theBackup.FileName)

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
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", theBackup.FileName))
	c.Header("Content-Type", objectInfo.ContentType)
	c.Header("Content-Length", fmt.Sprintf("%d", objectInfo.Size))

	// Stream the file to the response
	c.DataFromReader(http.StatusOK, objectInfo.Size, objectInfo.ContentType, object, nil)
}

func (h *AccountAgentHandler) API_DownloadAgentSave(c *gin.Context) {
	JWTData, _ := c.Keys["SessionJWT"].(app.Middleware_Session_JWT)
	AccountID := JWTData.AccountID
	AgentID := c.Param("agentid")
	SaveUUID := c.Param("uuid")

	theAccount, err := services.GetAccount(AccountID)
	if err != nil {
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

	var theSave models.AgentSave
	for _, save := range theAgent.Saves {
		if save.UUID == SaveUUID {
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

func (h *AccountAgentHandler) API_DownloadAgentLog(c *gin.Context) {
	JWTData, _ := c.Keys["SessionJWT"].(app.Middleware_Session_JWT)
	AccountID := JWTData.AccountID
	AgentID := c.Param("agentid")
	Type := c.Param("type")

	theAccount, err := services.GetAccount(AccountID)
	if err != nil {
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

	theLog, err := theAgent.GetLogOfType(Type)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	objectPath := fmt.Sprintf("%s/%s/logs/%s", theAccount.ID.Hex(), theAgent.ID.Hex(), theLog.FileName)

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
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", theLog.FileName))
	c.Header("Content-Type", objectInfo.ContentType)
	c.Header("Content-Length", fmt.Sprintf("%d", objectInfo.Size))

	// Stream the file to the response
	c.DataFromReader(http.StatusOK, objectInfo.Size, objectInfo.ContentType, object, nil)
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

	router.GET("/", handler.API_GetAllAgents)
	router.POST("/", handler.API_CreateNewAgent)

	router.GET("/:agentid", handler.API_GetAgentByID)
	router.DELETE("/:agentid", handler.API_DeleteAgent)

	router.PUT("/:agentid/configs", handler.API_UpdateAgentConfigs)

	router.GET("/:agentid/logs", handler.API_GetAgentLogs)

	router.GET("/:agentid/tasks", handler.API_GetAgentTasks)
	router.POST("/:agentid/tasks", handler.API_NewAgentTask)

	router.POST("/:agentid/mods/install", handler.API_AgentInstallMod)
	router.POST("/:agentid/mods/update", handler.API_AgentUpdateMod)
	router.POST("/:agentid/mods/uninstall", handler.API_AgentUninstallMod)

	router.GET("/:agentid/download/backup/:uuid", handler.API_DownloadAgentBackup)
	router.GET("/:agentid/download/save/:uuid", handler.API_DownloadAgentSave)
	router.GET("/:agentid/download/log/:type", handler.API_DownloadAgentLog)

	uploadGroup := router.Group("upload")
	uploadGroup.Use(middleware.Middleware_UploadFile())

	uploadGroup.POST("/:agentid/save", handler.API_UploadAgentSave)
}
