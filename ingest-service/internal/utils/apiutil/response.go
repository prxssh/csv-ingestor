package apiutil

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func Error(c *gin.Context, status int, message string) {
	c.JSON(status, gin.H{
		"status":  "error",
		"message": message,
	})
}

func InternalError(c *gin.Context) {
	Error(c, http.StatusInternalServerError, "internal error")
}

func Success(c *gin.Context, status int, data any) {
	c.JSON(status, gin.H{
		"status": "success",
		"data":   data,
	})
}
