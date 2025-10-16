package handlers

import (
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/middleware"
	"github.com/gin-gonic/gin"
)

type AccountHandler struct{}

func NewAccountHandler(router *gin.RouterGroup) {

	userGroup := router.Group("users")
	agentGroup := router.Group("agents")
	integrationsGroup := router.Group("integrations")

	router.Use(middleware.Middleware_DecodeJWT())
	router.Use(middleware.Middleware_VerifySession())

	NewAccountAgentHandler(agentGroup)
	NewAccountUserHandler(userGroup)
	NewAccountIntegrationsHandler(integrationsGroup)
}
