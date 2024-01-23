package routes

import (
	"net/http"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/services"
	"github.com/gin-gonic/gin"
)

func (obj *routes) AddAccountRoutes() {
	obj.AccountGroup.POST("/login", API_AccountLogin)
}

func API_AccountLogin(c *gin.Context) {
	var PostData API_AccountLogin_PostData
	if err := c.BindJSON(&PostData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	session, err := services.LoginAccountUser(PostData.Email, PostData.Password)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "sessionId": session.ID})
}
