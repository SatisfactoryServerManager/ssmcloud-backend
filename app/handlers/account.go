package handlers

import (
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/middleware"
	"github.com/gin-gonic/gin"
)

type AccountHandler struct{}

func NewAccountHandler(router *gin.RouterGroup) {

	agentGroup := router.Group("agents")

	router.Use(middleware.Middleware_DecodeJWT())
	router.Use(middleware.Middleware_VerifySession())

	NewAccountAgentHandler(agentGroup)
}
