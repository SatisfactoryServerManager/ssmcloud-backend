package handlers

import (
	"net/http"
	"os"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/models"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/utils/config"
	"github.com/gin-gonic/gin"
	"github.com/mrhid6/go-mongoose/mongoose"
	"go.mongodb.org/mongo-driver/bson"
)

type V1Handler struct{}

func (h *V1Handler) API_Ping(c *gin.Context) {

	hostname, _ := os.Hostname()
	configData, _ := config.GetConfigData()

	c.JSON(200, gin.H{"success": true, "node": hostname, "version": configData.Version})
}

func (h *V1Handler) API_Mods(c *gin.Context) {

	mods := make([]models.Mods, 0)

	if err := mongoose.FindAll(bson.M{}, &mods); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	c.JSON(200, gin.H{"success": true, "mods": mods})
}

func NewV1Handler(router *gin.RouterGroup) {
	handler := V1Handler{}

	router.GET("/ping", handler.API_Ping)
	router.GET("/mods", handler.API_Mods)

	NewAgentHandler(router.Group("agents"))
	NewAccountHandler(router.Group("account"))
	NewSSMModHandler(router.Group("ssmmod"))
}
