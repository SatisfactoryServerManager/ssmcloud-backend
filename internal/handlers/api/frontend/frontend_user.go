package frontend

import (
	"github.com/gin-gonic/gin"
)

type FrontendUserHandler struct{}

func NewFrontendUserHandler(router *gin.RouterGroup) {

	meGroup := router.Group("me")

	NewFrontendUserAccountHandler(meGroup)
}
