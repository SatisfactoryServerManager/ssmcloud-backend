package handlers

import (
	"net/http"

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

func NewFrontendUserAccountHandler(router *gin.RouterGroup) {

	handler := FrontendUserAccountHandler{}

	router.GET("/accounts", handler.API_GetMyLinkedAccounts)
	router.POST("/accounts", handler.API_CreateAccount)
	router.POST("/accounts/join", handler.API_JoinAccount)
	router.GET("/accounts/switch", handler.API_SwitchAccount)

	accountGroup := router.Group("account")
	accountGroup.GET("/", handler.API_GetMyAccount)
	accountGroup.GET("/audit", handler.API_GetMyAccountAudit)
	accountGroup.GET("/users", handler.API_GetMyAccountUsers)

	agentsGroup := accountGroup.Group("agents")

	NewFrontendUserAccountAgentHandler(agentsGroup)
}
