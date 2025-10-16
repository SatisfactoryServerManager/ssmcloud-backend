package handlers

import (
	"net/http"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/app"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/middleware"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/services"
	"github.com/gin-gonic/gin"
)

type AccountUserHandler struct{}

func (h *AccountUserHandler) API_CreateAPIKey(c *gin.Context) {
	JWTData, _ := c.Keys["SessionJWT"].(app.Middleware_Session_JWT)
	AccountID := JWTData.AccountID
	UserID := JWTData.UserID

	var PostData app.API_AccountUserApiKey_PostData
	if err := c.BindJSON(&PostData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	err := services.CreateUserAPIKey(AccountID, UserID, PostData.APIKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *AccountUserHandler) API_DeleteAPIKey(c *gin.Context) {
	JWTData, _ := c.Keys["SessionJWT"].(app.Middleware_Session_JWT)
	AccountID := JWTData.AccountID
	UserID := JWTData.UserID

	ShortKey := c.Param("shortkey")

	err := services.DeleteUserAPIKey(AccountID, UserID, ShortKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func NewAccountUserHandler(router *gin.RouterGroup) {
	handler := AccountUserHandler{}

	router.Use(middleware.Middleware_DecodeJWT())
	router.Use(middleware.Middleware_VerifySession())

	router.POST("/me/apikey", handler.API_CreateAPIKey)
	router.DELETE("/me/apikey/:shortkey", handler.API_DeleteAPIKey)

}
