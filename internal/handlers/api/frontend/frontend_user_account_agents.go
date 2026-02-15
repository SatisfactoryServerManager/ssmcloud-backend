package frontend

import (
	"net/http"

	v2 "github.com/SatisfactoryServerManager/ssmcloud-backend/internal/services/v2"
	models "github.com/SatisfactoryServerManager/ssmcloud-resources/models/v2"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type FrontendUserAccountAgentsHandler struct{}

func (handler *FrontendUserAccountAgentsHandler) API_GetMyAccountAgents(c *gin.Context) {
	claims, _ := c.Get("user")
	user := claims.(jwt.MapClaims)
	eid := user["sub"].(string)

	id := c.Query("_id")
	var oid primitive.ObjectID
	if id != "" {
		oid, _ = primitive.ObjectIDFromHex(id)
	}

	theUser, err := v2.GetMyUser(primitive.ObjectID{}, eid, "", "")

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	account, err := v2.GetMyUserAccount(theUser)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	agents, err := v2.GetMyUserAccountAgents(account, oid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "error": "", "agents": agents})
}

func (handler *FrontendUserAccountAgentsHandler) API_CreateAgent(c *gin.Context) {
	claims, _ := c.Get("user")
	user := claims.(jwt.MapClaims)
	eid := user["sub"].(string)

	theUser, err := v2.GetMyUser(primitive.ObjectID{}, eid, "", "")

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	account, err := v2.GetMyUserAccount(theUser)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	PostData := &models.CreateAgentWorkflowData{}
	if err := c.BindJSON(PostData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	if PostData.AgentName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "agent name is empty", "success": false})
		c.Abort()
		return
	}

	workflowId, err := v2.CreateAgentWorkflow(account.ID, PostData)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "error": "", "workflow_id": workflowId})
}

func (handler *FrontendUserAccountAgentsHandler) API_DeleteAgent(c *gin.Context) {

	claims, _ := c.Get("user")
	user := claims.(jwt.MapClaims)
	eid := user["sub"].(string)

	id := c.Query("_id")
	var oid primitive.ObjectID

	if id == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "id was empty", "success": false})
		c.Abort()
		return
	}

	oid, err := primitive.ObjectIDFromHex(id)
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

	account, err := v2.GetMyUserAccount(theUser)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	if err := v2.DeleteAgent(account, oid); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "error": ""})
}

func NewFrontendUserAccountAgentHandler(router *gin.RouterGroup) {

	handler := FrontendUserAccountAgentsHandler{}

	router.GET("/", handler.API_GetMyAccountAgents)
	router.POST("/", handler.API_CreateAgent)
	router.DELETE("/", handler.API_DeleteAgent)

}
