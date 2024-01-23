package routes

import "github.com/gin-gonic/gin"

func (obj *routes) AddV1Routes() {
	obj.MainGroup.GET("/ping", API_Ping)
	obj.AddAccountRoutes()
}

func API_Ping(c *gin.Context) {
	c.JSON(200, gin.H{"success": true})
}
