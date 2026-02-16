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

	theUser, err := v2.GetUser(primitive.ObjectID{}, eid, "", "")

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	theAccount, err := v2.GetUserActiveAccount(theUser)

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

// func (hander *FrontendUserAccountHandler) API_DeleteAccountIntegration(c *gin.Context) {

// 	accountId, _ := primitive.ObjectIDFromHex(c.Query("accountId"))
// 	id, _ := primitive.ObjectIDFromHex(c.Query("integrationId"))

// 	err := v2.DeleteAccountIntegration(id)
// 	if err != nil {
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
// 		c.Abort()
// 		return
// 	}

// 	c.JSON(http.StatusOK, gin.H{"success": true, "error": ""})
// }

func NewFrontendUserAccountHandler(router *gin.RouterGroup) {

	handler := FrontendUserAccountHandler{}

	accountGroup := router.Group("account")
	accountGroup.POST("/integrations/add", handler.API_AddAccountIntegrations)
	accountGroup.PUT("/integrations/update", handler.API_UpdateAccountIntegration)
	//accountGroup.DELETE("/integrations/delete", handler.API_DeleteAccountIntegration)
}
