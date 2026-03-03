package movieapi

import "github.com/gin-gonic/gin"

func (h *Handler) InitV1Routes(r *gin.RouterGroup) {
	r.GET("/", h.ListMovies)
	r.GET("/:id", h.GetMovie)
}
