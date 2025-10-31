package middleware

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/repositories"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/services"
	v2 "github.com/SatisfactoryServerManager/ssmcloud-resources/models/v2"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

var (
	jwks keyfunc.Keyfunc
)

func init() {
	godotenv.Load(".env.local")
	issuer := os.Getenv("AUTHENTIK_URL") + "/" + os.Getenv("AUTHENTIK_APPLICATION")

	jwksURL := issuer + "/jwks/"
	var err error
	jwks, err = keyfunc.NewDefaultCtx(context.Background(), []string{jwksURL}) // Context is used to end the refresh goroutine.
	if err != nil {
		log.Fatalf("Failed to create a keyfunc.Keyfunc from the server's URL.\nError: %s", err)
	}

	if err != nil {
		panic(fmt.Sprintf("failed to get JWKS: %v", err))
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

		AgentModel, err := repositories.GetMongoClient().GetModel("Agent")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
			c.Abort()
			return
		}

		theAgent := &v2.AgentSchema{}

		if err := AgentModel.FindOne(theAgent, bson.M{"apiKey": key}); err != nil {
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

		token, err := jwt.Parse(tokenStr, jwks.Keyfunc, jwt.WithLeeway(30*time.Second))
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
