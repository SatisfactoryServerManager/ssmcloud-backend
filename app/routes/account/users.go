package account

import (
	"net/http"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/app"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/services"
	"github.com/gin-gonic/gin"
)

func API_GetAllUsers(c *gin.Context) {
	JWTData, _ := c.Keys["SessionJWT"].(app.Middleware_Session_JWT)
	AccountID := JWTData.AccountID

	users, err := services.GetAllUsers(AccountID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "users": users})

}

func API_GetMyUser(c *gin.Context) {
	JWTData, _ := c.Keys["SessionJWT"].(app.Middleware_Session_JWT)
	AccountID := JWTData.AccountID
	UserID := JWTData.UserID

	user, err := services.GetMyUser(AccountID, UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "user": user})
}

func API_GetUserByInviteCode(c *gin.Context) {
	inviteCode := c.Param("invitecode")

	if inviteCode == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invite code was nil", "success": false})
		c.Abort()
		return
	}

	user, err := services.GetUserByInviteCode(inviteCode)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "user": user})
}

func API_GenerateUserTwoFASecret(c *gin.Context) {
	JWTData, _ := c.Keys["SessionJWT"].(app.Middleware_Session_JWT)
	AccountID := JWTData.AccountID
	UserID := JWTData.UserID

	secret, err := services.GenerateUserTwoFASecret(AccountID, UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "secret": secret})
}

func API_ValidateUserTwoFACode(c *gin.Context) {
	JWTData, _ := c.Keys["SessionJWT"].(app.Middleware_Session_JWT)
	AccountID := JWTData.AccountID
	UserID := JWTData.UserID

	var PostData app.API_AccountUserTwoFACode_PostData
	if err := c.BindJSON(&PostData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	err := services.ValidateUserTwoFACode(AccountID, UserID, PostData.Token)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func API_CreateAccountUser(c *gin.Context) {
	JWTData, _ := c.Keys["SessionJWT"].(app.Middleware_Session_JWT)
	AccountID := JWTData.AccountID

	var PostData app.API_CreateAccountUser_PostData
	if err := c.BindJSON(&PostData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	if err := services.CreateAccountUser(AccountID, PostData.Email); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func API_AcceptUserInvite(c *gin.Context) {
	inviteCode := c.Param("invitecode")

	if inviteCode == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invite code was nil", "success": false})
		c.Abort()
		return
	}

	var PostData app.API_AcceptInvite_PostData
	if err := c.BindJSON(&PostData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	if err := services.AcceptInviteCode(inviteCode, PostData.Password); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}
