package routes

import (
	"net/http"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/models"
	"github.com/gin-gonic/gin"
	"github.com/mrhid6/go-mongoose/mongoose"
	"go.mongodb.org/mongo-driver/bson"
)

func API_Ping(c *gin.Context) {
	c.JSON(200, gin.H{"success": true})
}

func API_Mods(c *gin.Context) {

	mods := make([]models.Mods, 0)

	if err := mongoose.FindAll(bson.M{}, &mods); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	c.JSON(200, gin.H{"success": true, "mods": mods})
}
