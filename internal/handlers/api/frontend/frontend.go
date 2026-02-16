package frontend

import (
	"fmt"
	"net/http"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/middleware"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/repositories"
	v2 "github.com/SatisfactoryServerManager/ssmcloud-backend/internal/services/v2"
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

	// Validate agent ID
	id := c.Query("agentid")
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid agent id", "success": false})
		return
	}

	uuid := c.Query("uuid")
	if uuid == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing uuid", "success": false})
		return
	}

	// Get user
	theUser, err := v2.GetUser(primitive.ObjectID{}, eid, "", "")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		return
	}

	// Get account
	theAccount, err := v2.GetUserActiveAccount(theUser)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		return
	}

	// Get agent
	agents, err := v2.GetUserAccountAgents(theAccount, oid)
	if err != nil || len(agents) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "agent not found", "success": false})
		return
	}
	theAgent := agents[0]

	// Find backup
	var theBackup *models.AgentBackup
	for idx := range theAgent.Backups {
		if theAgent.Backups[idx].UUID == uuid {
			theBackup = &theAgent.Backups[idx]
			break
		}
	}

	if theBackup == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "backup not found", "success": false})
		return
	}

	if theBackup.FileName == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "backup file has no filename", "success": false})
		return
	}

	// Build object path
	objectPath := fmt.Sprintf("%s/%s/backups/%s",
		theAccount.ID.Hex(),
		theAgent.ID.Hex(),
		theBackup.FileName,
	)

	fmt.Println("Downloading:", objectPath)

	// --- AWS S3 GetObject ---
	obj, err := repositories.GetAgentFile(objectPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		return
	}
	defer obj.Body.Close()

	// Determine metadata
	size := obj.ContentLength
	contentType := "application/octet-stream"
	if obj.ContentType != nil {
		contentType = *obj.ContentType
	}

	// Force download
	c.Header("Content-Disposition",
		fmt.Sprintf(`attachment; filename="%s"`, theBackup.FileName),
	)
	c.Header("Content-Type", contentType)
	c.Header("Content-Length", fmt.Sprintf("%d", *size))

	// Stream S3 object to client
	c.DataFromReader(http.StatusOK, *size, contentType, obj.Body, nil)
}

func (handler *FrontendHandler) API_DownloadSave(c *gin.Context) {
	claims, _ := c.Get("user")
	user := claims.(jwt.MapClaims)
	eid := user["sub"].(string)

	id := c.Query("agentid")
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		return
	}

	uuid := c.Query("uuid")

	theUser, err := v2.GetUser(primitive.ObjectID{}, eid, "", "")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		return
	}

	theAccount, err := v2.GetUserActiveAccount(theUser)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		return
	}

	agents, err := v2.GetUserAccountAgents(theAccount, oid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		return
	}
	theAgent := agents[0]

	// Find the save
	var theSave *models.AgentSave
	for idx := range theAgent.Saves {
		save := &theAgent.Saves[idx]
		if save.UUID == uuid {
			theSave = save
			break
		}
	}

	if theSave == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "cant find save on agent", "success": false})
		return
	}

	if theSave.FileName == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "save file not found", "success": false})
		return
	}

	objectPath := fmt.Sprintf("%s/%s/saves/%s",
		theAccount.ID.Hex(),
		theAgent.ID.Hex(),
		theSave.FileName,
	)

	// --- AWS S3 GetObject ---
	obj, err := repositories.GetAgentFile(objectPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		return
	}
	defer obj.Body.Close()

	// Determine metadata
	size := obj.ContentLength
	contentType := "application/octet-stream"
	if obj.ContentType != nil {
		contentType = *obj.ContentType
	}

	// Force download
	c.Header("Content-Disposition",
		fmt.Sprintf(`attachment; filename="%s"`, theSave.FileName),
	)
	c.Header("Content-Type", contentType)
	c.Header("Content-Length", fmt.Sprintf("%d", size))

	// Stream S3 object to client
	c.DataFromReader(http.StatusOK, *size, contentType, obj.Body, nil)
}

func (handler *FrontendHandler) API_DownloadLog(c *gin.Context) {
	claims, _ := c.Get("user")
	user := claims.(jwt.MapClaims)
	eid := user["sub"].(string)

	id := c.Query("agentid")
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		return
	}

	Type := c.Query("logtype")

	AgentModel, err := repositories.GetMongoClient().GetModel("Agent")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		return
	}

	theUser, err := v2.GetUser(primitive.ObjectID{}, eid, "", "")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		return
	}

	theAccount, err := v2.GetUserActiveAccount(theUser)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		return
	}

	agents, err := v2.GetUserAccountAgents(theAccount, oid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		return
	}
	theAgent := agents[0]

	// Load Logs field
	if err := AgentModel.PopulateField(theAgent, "Logs"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		return
	}

	var theLog *models.AgentLogSchema
	for idx := range theAgent.Logs {
		log := &theAgent.Logs[idx]
		if log.Type == Type {
			theLog = log
			break
		}
	}

	if theLog == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "cant find log on agent", "success": false})
		return
	}

	if theLog.FileName == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "log file not found", "success": false})
		return
	}

	objectPath := fmt.Sprintf("%s/%s/logs/%s",
		theAccount.ID.Hex(),
		theAgent.ID.Hex(),
		theLog.FileName,
	)

	// --- AWS S3 download ---
	obj, err := repositories.GetAgentFile(objectPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		return
	}
	defer obj.Body.Close()

	// Extract metadata
	size := obj.ContentLength
	contentType := "text/plain"
	if obj.ContentType != nil {
		contentType = *obj.ContentType
	}

	// --- Download headers ---
	c.Header("Content-Disposition",
		fmt.Sprintf(`attachment; filename="%s"`, theLog.FileName),
	)
	c.Header("Content-Type", contentType)
	c.Header("Content-Length", fmt.Sprintf("%d", size))

	// --- Stream the log file ---
	c.DataFromReader(http.StatusOK, *size, contentType, obj.Body, nil)
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
