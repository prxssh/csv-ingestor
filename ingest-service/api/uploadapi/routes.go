package uploadapi

import "github.com/gin-gonic/gin"

func (h *Handler) InitV1Routes(r *gin.RouterGroup) {
	r.POST("/multipart/init", h.InitMultipartUpload)
	r.GET("/multipart/:id/presign", h.GetPresignedParts)
	r.PATCH("/multipart/:id/part", h.ReportPartUploaded)
	r.POST("/multipart/:id/complete", h.CompleteMultipartUpload)
	r.DELETE("/multipart/:id/abort", h.AbortMultipartUpload)

	r.GET("/:id/status", h.GetUploadStatus)
}
