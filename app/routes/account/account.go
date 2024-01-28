package account

import (
	"net/http"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/app"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/services"
	"github.com/gin-gonic/gin"
)



func API_AccountLogin(c *gin.Context) {
	var PostData app.API_AccountLogin_PostData
	if err := c.BindJSON(&PostData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	sessionJWT, err := services.LoginAccountUser(PostData.Email, PostData.Password)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "session": sessionJWT})
}

func API_AccountSignUp(c *gin.Context) {
	var PostData app.API_AccountSignup_PostData
	if err := c.BindJSON(&PostData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	err := services.AccountSignup(PostData.AccountName, PostData.Email, PostData.Password)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func API_AccountSession(c *gin.Context) {

	JWTData, _ := c.Keys["SessionJWT"].(app.Middleware_Session_JWT)
	SessionID := JWTData.SessionID

	session, err := services.GetAccountSession(SessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "session": session})
}
