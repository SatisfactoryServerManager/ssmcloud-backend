package routes

import (
	"net/http"
	"os"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/app"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/services"
	"github.com/gin-gonic/gin"
	"github.com/kataras/jwt"
)

func Middleware_DecodeJWT() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.GetHeader("x-ssm-jwt")

		if key == "" {
			c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "JWT Key was null!", "key": key})
			c.Abort()
			return
		}

		verifiedToken, err := jwt.Verify(jwt.HS256, []byte(os.Getenv("JWT_KEY")), []byte(key))
		if err != nil {
			c.JSON(http.StatusForbidden, gin.H{"success": false, "error": err.Error()})
			c.Abort()
			return
		}

		var claims app.Middleware_Session_JWT
		err = verifiedToken.Claims(&claims)
		if err != nil {
			c.JSON(http.StatusForbidden, gin.H{"success": false, "error": err.Error()})
			c.Abort()
			return
		}

		c.Set("SessionJWT", claims)

		c.Next()
	}
}

func Middleware_VerifySession() gin.HandlerFunc {
	return func(c *gin.Context) {
		JWTData, _ := c.Keys["SessionJWT"].(app.Middleware_Session_JWT)
		SessionID := JWTData.SessionID

		_, err := services.GetAccountSession(SessionID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
			c.Abort()
			return
		}

		c.Next()
	}
}
