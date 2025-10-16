package handlers

import (
	"net/http"
	"os"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/middleware"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/utils/config"
	"github.com/SatisfactoryServerManager/ssmcloud-resources/models"
	modelsv1 "github.com/SatisfactoryServerManager/ssmcloud-resources/models/v1"
	"github.com/gin-gonic/gin"
	"github.com/mrhid6/go-mongoose/mongoose"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type V1Handler struct{}

func (h *V1Handler) API_Ping(c *gin.Context) {

	hostname, _ := os.Hostname()
	configData, _ := config.GetConfigData()

	c.JSON(200, gin.H{"success": true, "node": hostname, "version": configData.Version})
}

func (h *V1Handler) API_Mods(c *gin.Context) {

	mods := make([]models.Mods, 0)

	if err := mongoose.FindAll(bson.M{}, &mods); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	c.JSON(200, gin.H{"success": true, "mods": mods})
}

func (h *V1Handler) API_GetWorkflow(c *gin.Context) {
	WorkflowIDString := c.Param("workflowId")
	WorkflowId, err := primitive.ObjectIDFromHex(WorkflowIDString)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	var theWorkflow modelsv1.Workflows

	if err := mongoose.FindOne(bson.M{"_id": WorkflowId}, &theWorkflow); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	c.JSON(200, gin.H{"success": true, "workflow": theWorkflow})
}

func NewV1Handler(router *gin.RouterGroup) {
	group := router.Group("v1")
	handler := V1Handler{}

	AgentGroup := group.Group("agent")
	AccountGroup := group.Group("account")
	SSMModGroup := group.Group("ssmmod")

	group.GET("/ping", handler.API_Ping)
	group.GET("/mods", handler.API_Mods)

	group.Use(middleware.Middleware_DecodeJWT())
	group.Use(middleware.Middleware_VerifySession())

	group.GET("/workflows/:workflowId", handler.API_GetWorkflow)

	NewAgentHandler(AgentGroup)
	NewAccountHandler(AccountGroup)
	NewSSMModHandler(SSMModGroup)
}
