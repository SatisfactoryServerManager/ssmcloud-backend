package handlers

import (
	"net/http"
	"os"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/repositories"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/utils/config"
	"github.com/SatisfactoryServerManager/ssmcloud-resources/models"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
)

type V1Handler struct{}

func (h *V1Handler) API_Ping(c *gin.Context) {

	hostname, _ := os.Hostname()
	configData, _ := config.GetConfigData()

	c.JSON(200, gin.H{"success": true, "node": hostname, "version": configData.Version})
}

func (h *V1Handler) API_Mods(c *gin.Context) {

	ModModel, err := repositories.GetMongoClient().GetModel("Mod")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	mods := make([]models.ModSchema, 0)

	if err := ModModel.FindAll(&mods, bson.M{}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "success": false})
		c.Abort()
		return
	}

	c.JSON(200, gin.H{"success": true, "mods": mods})
}

func NewV1Handler(router *gin.RouterGroup) {
	group := router.Group("v1")
	handler := V1Handler{}

	AgentGroup := group.Group("agent")
	SSMModGroup := group.Group("ssmmod")

	group.GET("/ping", handler.API_Ping)
	group.GET("/mods", handler.API_Mods)

	NewAgentHandler(AgentGroup)
	NewSSMModHandler(SSMModGroup)
}
