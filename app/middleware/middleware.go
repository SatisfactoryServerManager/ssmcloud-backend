package middleware

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/MicahParks/keyfunc"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/services"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/types"
	models "github.com/SatisfactoryServerManager/ssmcloud-resources/models/v1"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"github.com/joho/godotenv"
	jwt2 "github.com/kataras/jwt"
	"github.com/mrhid6/go-mongoose/mongoose"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

var (
	jwks *keyfunc.JWKS
)

func init() {
	godotenv.Load(".env.local")
	issuer := os.Getenv("AUTHENTIK_URL") + "/" + os.Getenv("AUTHENTIK_APPLICATION")

	jwksURL := issuer + "/jwks/"
	var err error
	jwks, err = keyfunc.Get(jwksURL, keyfunc.Options{
		RefreshInterval:   time.Hour,
		RefreshUnknownKID: true,
	})
	if err != nil {
		panic(fmt.Sprintf("failed to get JWKS: %v", err))
	}
}

func Middleware_DecodeJWT() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.GetHeader("x-ssm-jwt")

		if key == "" {
			c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "JWT Key was null!", "key": key})
			c.Abort()
			return
		}

		verifiedToken, err := jwt2.Verify(jwt2.HS256, []byte(os.Getenv("JWT_KEY")), []byte(key))
		if err != nil {
			c.JSON(http.StatusForbidden, gin.H{"success": false, "error": err.Error()})
			c.Abort()
			return
		}

		var claims types.Middleware_Session_JWT
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
		JWTData, _ := c.Keys["SessionJWT"].(types.Middleware_Session_JWT)
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

func Middleware_AgentAPIKey() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.GetHeader("x-ssm-key")

		if key == "" {
			c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "API Key was null!", "key": key})
			c.Abort()
			return
		}

		var theAgent models.Agents

		if err := mongoose.FindOne(bson.M{"apiKey": key}, &theAgent); err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "Invalid API Key", "key": key})
				c.Abort()
				return
			}
			c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "API Key was null!", "key": key})
			c.Abort()
			return
		}

		c.Set("AgentKey", key)
	}
}

func Middleware_UploadFile() gin.HandlerFunc {
	return func(c *gin.Context) {
		file, _ := c.FormFile("file")
		fileIdentity := services.ConvertUploadToFileIdentity(file)

		c.SaveUploadedFile(file, fileIdentity.LocalFilePath)
		c.Set("FileIdentity", fileIdentity)
	}
}

func Middleware_CheckFrontendAPIKey() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.GetHeader("apikey")

		if key == "" {
			c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "API Key was null!", "key": key})
			c.Abort()
			return
		}

		if key != os.Getenv("SECRET_KEY") {
			c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "API Key mismatch!", "key": key})
			c.Abort()
			return
		}
	}
}

func Middleware_CheckFrontendAccessToken() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr := c.GetHeader("x-ssm-auth-token")

		if tokenStr == "" {
			c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "access token was null!", "key": tokenStr})
			c.Abort()
			return
		}

		token, err := jwt.Parse(tokenStr, jwks.Keyfunc)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": fmt.Sprintf("invalid token: %v", err)})
			c.Abort()
			return
		}

		if !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "invalid token"})
			c.Abort()
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "invalid token claims"})
			c.Abort()
			return
		}

		c.Set("user", claims)
	}
}
