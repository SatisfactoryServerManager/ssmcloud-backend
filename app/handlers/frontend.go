package handlers

import (
	"fmt"
	"net/http"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/middleware"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/repositories"
	v2 "github.com/SatisfactoryServerManager/ssmcloud-backend/app/services/v2"
	models "github.com/SatisfactoryServerManager/ssmcloud-resources/models/v2"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type FrontendHandler struct{}

func (hander *FrontendHandler) API_GetWorkflows(c *gin.Context) {

	WorkflowModel, err := repositories.GetMongoClient().GetModel("Workflow")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	WorkflowIDString := c.Query("workflowId")

	WorkflowId, err := primitive.ObjectIDFromHex(WorkflowIDString)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	var theWorkflow models.WorkflowSchema

	if err := WorkflowModel.FindOne(&theWorkflow, bson.M{"_id": WorkflowId}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	c.JSON(200, gin.H{"success": true, "workflow": theWorkflow})
}

func (handler *FrontendHandler) API_DownloadBackup(c *gin.Context) {
	claims, _ := c.Get("user")
	user := claims.(jwt.MapClaims)
	eid := user["sub"].(string)

	id := c.Query("agentid")
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	uuid := c.Query("uuid")

	theUser, err := v2.GetMyUser(primitive.ObjectID{}, eid, "", "")

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	theAccount, err := v2.GetMyUserAccount(theUser)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	agents, err := v2.GetMyUserAccountAgents(theAccount, oid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}
	theAgent := agents[0]

	var theBackup *models.AgentBackup
	for idx := range theAgent.Backups {
		backup := &theAgent.Backups[idx]
		if backup.UUID == uuid {
			theBackup = backup
		}
	}

	if theBackup == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "cant find backup on agent", "success": false})
		c.Abort()
		return
	}

	if theBackup.FileName == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "backup file not found", "success": false})
		c.Abort()
		return
	}

	objectPath := fmt.Sprintf("%s/%s/backups/%s", theAccount.ID.Hex(), theAgent.ID.Hex(), theBackup.FileName)
	fmt.Println(objectPath)

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

func (handler *FrontendHandler) API_DownloadSave(c *gin.Context) {
	claims, _ := c.Get("user")
	user := claims.(jwt.MapClaims)
	eid := user["sub"].(string)

	id := c.Query("agentid")
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	uuid := c.Query("uuid")

	theUser, err := v2.GetMyUser(primitive.ObjectID{}, eid, "", "")

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	theAccount, err := v2.GetMyUserAccount(theUser)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	agents, err := v2.GetMyUserAccountAgents(theAccount, oid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}
	theAgent := agents[0]

	var theSave *models.AgentSave
	for idx := range theAgent.Saves {
		save := &theAgent.Saves[idx]
		if save.UUID == uuid {
			theSave = save
		}
	}

	if theSave == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "cant find save on agent", "success": false})
		c.Abort()
		return
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

func (handler *FrontendHandler) API_DownloadLog(c *gin.Context) {
	claims, _ := c.Get("user")
	user := claims.(jwt.MapClaims)
	eid := user["sub"].(string)

	id := c.Query("agentid")
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	Type := c.Query("logtype")

	AgentModel, err := repositories.GetMongoClient().GetModel("Agent")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	theUser, err := v2.GetMyUser(primitive.ObjectID{}, eid, "", "")

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	theAccount, err := v2.GetMyUserAccount(theUser)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	agents, err := v2.GetMyUserAccountAgents(theAccount, oid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}
	theAgent := agents[0]

	if err := AgentModel.PopulateField(theAgent, "Logs"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	var theLog *models.AgentLogSchema
	for idx := range theAgent.Logs {
		log := &theAgent.Logs[idx]
		if log.Type == Type {
			theLog = log
		}
	}

	if theLog == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "cant find log on agent", "success": false})
		c.Abort()
		return
	}

	if theLog.FileName == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "log file not found", "success": false})
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

func NewFrontendHandler(router gin.RouterGroup) {

	handler := &FrontendHandler{}

	router.Use(middleware.Middleware_CheckFrontendAPIKey())
	router.Use(middleware.Middleware_CheckFrontendAccessToken())

	usersGroup := router.Group("users")
	NewFrontendUserHandler(usersGroup)

	router.GET("/workflows", handler.API_GetWorkflows)

	router.GET("/download/backup", handler.API_DownloadBackup)
	router.GET("/download/save", handler.API_DownloadSave)
	router.GET("/download/log", handler.API_DownloadLog)
}
