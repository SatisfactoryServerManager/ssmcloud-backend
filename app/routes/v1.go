package routes

import "github.com/gin-gonic/gin"

func API_Ping(c *gin.Context) {
	c.JSON(200, gin.H{"success": true})
}
