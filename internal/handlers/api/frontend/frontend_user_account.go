package frontend

import (
	"net/http"

	v2 "github.com/SatisfactoryServerManager/ssmcloud-backend/internal/services/v2"
	models "github.com/SatisfactoryServerManager/ssmcloud-resources/models/v2"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
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

func (hander *FrontendUserAccountHandler) API_GetMyAccountIntegrations(c *gin.Context) {
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

	integrations, err := v2.GetMyAccountIntegrations(theAccount)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "error": "", "integrations": integrations})
}

func (handler *FrontendUserAccountHandler) API_GetMyAccountIntegrationsEvents(c *gin.Context) {
	claims, _ := c.Get("user")
	user := claims.(jwt.MapClaims)
	eid := user["sub"].(string)

	integrationId, err := primitive.ObjectIDFromHex(c.Query("integrationId"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	_, err = v2.GetMyUser(primitive.ObjectID{}, eid, "", "")

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	events, err := v2.GetMyAccountIntegrationsEvents(integrationId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "error": "", "events": events})
}

func (handler *FrontendUserAccountHandler) API_AddAccountIntegrations(c *gin.Context) {

	claims, _ := c.Get("user")
	user := claims.(jwt.MapClaims)
	eid := user["sub"].(string)

	type APIPostAccountIntegrationsData struct {
		Name       string                        `json:"name"`
		Type       models.IntegrationType        `json:"type"`
		URL        string                        `json:"url"`
		EventTypes []models.IntegrationEventType `json:"eventTypes"`
	}

	PostData := &APIPostAccountIntegrationsData{}
	if err := c.BindJSON(PostData); err != nil {
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

	err = v2.AddAccountIntegration(theAccount, PostData.Name, PostData.Type, PostData.URL, PostData.EventTypes)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "error": ""})
}

func (hander *FrontendUserAccountHandler) API_UpdateAccountIntegration(c *gin.Context) {

	type APIPostAccountIntegrationsData struct {
		IntegrationId primitive.ObjectID            `json:"integrationId"`
		Name          string                        `json:"name"`
		Type          models.IntegrationType        `json:"type"`
		URL           string                        `json:"url"`
		EventTypes    []models.IntegrationEventType `json:"eventTypes"`
	}

	PostData := &APIPostAccountIntegrationsData{}
	if err := c.BindJSON(PostData); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	err := v2.UpdateAccountIntegration(PostData.IntegrationId, PostData.Name, PostData.Type, PostData.URL, PostData.EventTypes)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "error": ""})
}

func (hander *FrontendUserAccountHandler) API_DeleteAccountIntegration(c *gin.Context) {

	id, _ := primitive.ObjectIDFromHex(c.Query("integrationId"))

	err := v2.DeleteAccountIntegration(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "error": ""})
}

func NewFrontendUserAccountHandler(router *gin.RouterGroup) {

	handler := FrontendUserAccountHandler{}

	router.POST("/accounts", handler.API_CreateAccount)
	router.POST("/accounts/join", handler.API_JoinAccount)
	router.GET("/accounts/switch", handler.API_SwitchAccount)

	accountGroup := router.Group("account")
	accountGroup.GET("/", handler.API_GetMyAccount)
	accountGroup.GET("/integrations", handler.API_GetMyAccountIntegrations)
	accountGroup.GET("/integrations/events", handler.API_GetMyAccountIntegrationsEvents)
	accountGroup.POST("/integrations/add", handler.API_AddAccountIntegrations)
	accountGroup.PUT("/integrations/update", handler.API_UpdateAccountIntegration)
	accountGroup.DELETE("/integrations/delete", handler.API_DeleteAccountIntegration)

	agentsGroup := accountGroup.Group("agents")

	NewFrontendUserAccountAgentHandler(agentsGroup)
}
