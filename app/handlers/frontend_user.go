package handlers

import (
	"net/http"
	"time"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/repositories"
	v2 "github.com/SatisfactoryServerManager/ssmcloud-backend/app/services/v2"
	models "github.com/SatisfactoryServerManager/ssmcloud-resources/models/v2"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type FrontendUserHandler struct{}

func (handler *FrontendUserHandler) API_GetMyUser(c *gin.Context) {
	eid := c.Query("eid")
	email := c.Query("email")
	id := c.Query("_id")
	username := c.Query("username")

	claims, _ := c.Get("user")
	user := claims.(jwt.MapClaims)

	if user["email"] != nil {
		email = user["email"].(string)
	}
	if user["sub"] != nil {
		eid = user["sub"].(string)
	}
	if user["preferred_username"] != nil {
		username = user["preferred_username"].(string)
	}

	var oid primitive.ObjectID
	if id != "" {
		oid, _ = primitive.ObjectIDFromHex(id)
	}

	theUser, err := v2.GetMyUser(oid, eid, email, username)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	avatarUrl, ok := user["avatar"].(string)
	if ok {
		if err := v2.UpdateUserProfilePicture(theUser, avatarUrl); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
			c.Abort()
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "error": "", "user": theUser})
}

func (handler *FrontendUserHandler) API_CreateUser(c *gin.Context) {
	claims, _ := c.Get("user")
	user := claims.(jwt.MapClaims)

	email := user["email"].(string)
	sub := user["sub"].(string)
	username := user["preferred_username"].(string)

	UserModel, err := repositories.GetMongoClient().GetModel("User")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	NewUser := &models.UserSchema{
		ID:         primitive.NewObjectID(),
		ExternalID: sub,
		Email:      email,
		Username:   username,
		APIKeys:    make([]models.UserAPIKey, 0),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	if err := UserModel.Create(NewUser); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "error": "", "user": NewUser})
}

func NewFrontendUserHandler(router *gin.RouterGroup) {
	handler := FrontendUserHandler{}

	router.POST("/", handler.API_CreateUser)

	meGroup := router.Group("me")
	meGroup.GET("/", handler.API_GetMyUser)

	NewFrontendUserAccountHandler(meGroup)
}
