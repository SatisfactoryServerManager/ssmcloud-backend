package handlers

import (
	"net/http"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/app"
	v2 "github.com/SatisfactoryServerManager/ssmcloud-backend/app/services/v2"
	models "github.com/SatisfactoryServerManager/ssmcloud-resources/models/v2"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type FrontendUserAccountHandler struct{}

func (handler *FrontendUserAccountHandler) API_CreateAccount(c *gin.Context) {
	claims, _ := c.Get("user")
	user := claims.(jwt.MapClaims)
	eid := user["sub"].(string)

	theUser, err := v2.GetMyUser(primitive.ObjectID{}, eid, "", "")

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	type APINewAccountData struct {
		AccountName string `json:"accountName"`
	}

	PostData := &APINewAccountData{}
	if err := c.BindJSON(&PostData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		c.Abort()
		return
	}

	if PostData.AccountName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "account name is required"})
		c.Abort()
		return
	}

	if err := v2.CreateAccount(theUser, PostData.AccountName); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "error": ""})
}

func (handler *FrontendUserAccountHandler) API_JoinAccount(c *gin.Context) {
	claims, _ := c.Get("user")
	user := claims.(jwt.MapClaims)
	eid := user["sub"].(string)

	theUser, err := v2.GetMyUser(primitive.ObjectID{}, eid, "", "")

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	type APIJoinAccountData struct {
		JoinCode string `json:"joinCode"`
	}

	PostData := &APIJoinAccountData{}
	if err := c.BindJSON(&PostData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		c.Abort()
		return
	}

	if PostData.JoinCode == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "join code is required"})
		c.Abort()
		return
	}

	if err := v2.JoinAccount(theUser, PostData.JoinCode); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "error": ""})
}

func (handler *FrontendUserAccountHandler) API_SwitchAccount(c *gin.Context) {
	claims, _ := c.Get("user")
	user := claims.(jwt.MapClaims)
	eid := user["sub"].(string)

	theUser, err := v2.GetMyUser(primitive.ObjectID{}, eid, "", "")

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	id := c.Query("_id")
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	if err := v2.SwitchAccount(theUser, oid); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "error": ""})

}

func (handler *FrontendUserAccountHandler) API_GetMyAccount(c *gin.Context) {
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

	c.JSON(http.StatusOK, gin.H{"success": true, "error": "", "account": account})
}

func (handler *FrontendUserAccountHandler) API_GetMyAccountAudit(c *gin.Context) {
	claims, _ := c.Get("user")
	user := claims.(jwt.MapClaims)
	eid := user["sub"].(string)

	theUser, err := v2.GetMyUser(primitive.ObjectID{}, eid, "", "")

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	allAudits, err := v2.GetMyAccountAudit(theUser)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	filteredAudits := make([]models.AccountAuditSchema, 0)
	filter := c.Query("auditType")
	if filter != "" {
		for _, audit := range *allAudits {
			if audit.Type == filter {
				filteredAudits = append(filteredAudits, audit)
			}
		}
	} else {
		filteredAudits = *allAudits
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "error": "", "audit": filteredAudits})
}

func (handler *FrontendUserAccountHandler) API_GetMyAccountUsers(c *gin.Context) {
	claims, _ := c.Get("user")
	user := claims.(jwt.MapClaims)
	eid := user["sub"].(string)

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

	users, err := v2.GetMyAccountUsers(theAccount)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "error": "", "users": users})

}

func (handler *FrontendUserAccountHandler) API_GetMyLinkedAccounts(c *gin.Context) {
	claims, _ := c.Get("user")
	user := claims.(jwt.MapClaims)
	eid := user["sub"].(string)

	theUser, err := v2.GetMyUser(primitive.ObjectID{}, eid, "", "")

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	accounts, err := v2.GetMyUserLinkedAccounts(theUser)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "error": "", "accounts": accounts})
}

func (handler *FrontendUserAccountHandler) API_GetMyAccountAgents(c *gin.Context) {
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

func (handler *FrontendUserAccountHandler) API_CreateAgent(c *gin.Context) {
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

func (handler *FrontendUserAccountHandler) API_DeleteAgent(c *gin.Context) {

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

func (handler *FrontendUserAccountHandler) API_UpdateAgentSettings(c *gin.Context) {

	claims, _ := c.Get("user")
	user := claims.(jwt.MapClaims)
	eid := user["sub"].(string)

	PostData := &app.APIUpdateServerSettingsRequest{}
	if err := c.BindJSON(PostData); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	if PostData.ID == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "id was empty", "success": false})
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

	if err := v2.UpdateAgentSettings(account, PostData); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "error": ""})
}

func (handler *FrontendUserAccountHandler) API_GetAgentLog(c *gin.Context) {
	claims, _ := c.Get("user")
	user := claims.(jwt.MapClaims)
	eid := user["sub"].(string)

	id := c.Query("_id")
	logType := c.Query("type")

	if id == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "agent id is empty", "success": false})
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

	agents, err := v2.GetMyUserAccountAgents(account, oid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	if len(agents) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "agent was not found", "success": false})
		c.Abort()
		return
	}

	theAgent := agents[0]

	theLog, err := v2.GetAgentLog(theAgent, logType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "error": "", "agentLog": theLog})
}

func NewFrontendUserAccountHandler(router *gin.RouterGroup) {

	handler := FrontendUserAccountHandler{}

	router.GET("/accounts", handler.API_GetMyLinkedAccounts)
	router.POST("/accounts", handler.API_CreateAccount)
	router.POST("/accounts/join", handler.API_JoinAccount)
	router.GET("/accounts/switch", handler.API_SwitchAccount)
	router.GET("/account", handler.API_GetMyAccount)
	router.GET("/account/audit", handler.API_GetMyAccountAudit)
	router.GET("/account/users", handler.API_GetMyAccountUsers)
	router.GET("/account/agents", handler.API_GetMyAccountAgents)
	router.POST("/account/agents", handler.API_CreateAgent)
	router.DELETE("/account/agents", handler.API_DeleteAgent)
	router.POST("/account/agents/settings", handler.API_UpdateAgentSettings)
	router.GET("/account/agents/log", handler.API_GetAgentLog)
}
