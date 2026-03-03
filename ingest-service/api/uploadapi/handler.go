package uploadapi

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
	"github.com/prxssh/csv-ingestor/ingest-service/internal/upload"
	"github.com/prxssh/csv-ingestor/ingest-service/internal/utils/apiutil"
	"github.com/prxssh/csv-ingestor/ingest-service/internal/worker"
)

type Handler struct {
	uploadService *upload.Service
	asynqClient   *asynq.Client
}

func NewHandler(uploadService *upload.Service, asynqClient *asynq.Client) *Handler {
	return &Handler{uploadService: uploadService, asynqClient: asynqClient}
}

func (h *Handler) InitMultipartUpload(ctx *gin.Context) {
	var body upload.InitUploadRequest
	if err := ctx.ShouldBindJSON(&body); err != nil {
		apiutil.Error(ctx, http.StatusBadRequest, err.Error())
		return
	}

	resp, err := h.uploadService.InitUpload(ctx.Request.Context(), body)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "init upload failed", "error", err)
		apiutil.InternalError(ctx)
		return
	}

	apiutil.Success(ctx, http.StatusCreated, resp)
}

func (h *Handler) GetPresignedParts(ctx *gin.Context) {
	var uri jobIDURI
	if err := ctx.ShouldBindUri(&uri); err != nil {
		apiutil.Error(ctx, http.StatusBadRequest, err.Error())
		return
	}

	var query presignPartsQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		apiutil.Error(
			ctx,
			http.StatusBadRequest,
			"query param 'parts' is required (e.g. ?parts=1,2,3)",
		)
		return
	}

	partNumbers, err := parsePartNumbers(query.Parts)
	if err != nil {
		apiutil.Error(ctx, http.StatusBadRequest, "invalid part numbers: "+err.Error())
		return
	}

	resp, err := h.uploadService.GetPresignedParts(ctx.Request.Context(), uri.ID, partNumbers)
	if err != nil {
		switch {
		case errors.Is(err, upload.ErrJobNotFound):
			apiutil.Error(ctx, http.StatusNotFound, "upload job not found")
		case errors.Is(err, upload.ErrJobFinished):
			apiutil.Error(
				ctx,
				http.StatusConflict,
				"upload job is already completed or aborted",
			)
		default:
			slog.ErrorContext(
				ctx.Request.Context(),
				"get presigned parts failed",
				"job_id",
				uri.ID,
				"error",
				err,
			)
			apiutil.InternalError(ctx)
		}
		return
	}

	apiutil.Success(ctx, http.StatusOK, resp)
}

func (h *Handler) CompleteMultipartUpload(ctx *gin.Context) {
	var uri jobIDURI
	if err := ctx.ShouldBindUri(&uri); err != nil {
		apiutil.Error(ctx, http.StatusBadRequest, err.Error())
		return
	}

	var body upload.CompleteUploadRequest
	if err := ctx.ShouldBindJSON(&body); err != nil {
		apiutil.Error(ctx, http.StatusBadRequest, err.Error())
		return
	}

	resp, err := h.uploadService.CompleteUpload(ctx.Request.Context(), uri.ID, body)
	if err != nil {
		switch {
		case errors.Is(err, upload.ErrJobNotFound):
			apiutil.Error(ctx, http.StatusNotFound, "upload job not found")
		case errors.Is(err, upload.ErrJobAlreadyCompleted):
			apiutil.Error(ctx, http.StatusConflict, "upload job is already completed")
		case errors.Is(err, upload.ErrJobAborted):
			apiutil.Error(ctx, http.StatusConflict, "upload job has been aborted")
		default:
			slog.ErrorContext(
				ctx.Request.Context(),
				"complete upload failed",
				"job_id",
				uri.ID,
				"error",
				err,
			)
			apiutil.InternalError(ctx)
		}
		return
	}

	if err := worker.EnqueueProcessCSV(ctx.Request.Context(), h.asynqClient, uri.ID); err != nil {
		slog.ErrorContext(
			ctx.Request.Context(),
			"failed to enqueue csv processing",
			"job_id",
			uri.ID,
			"error",
			err,
		)
	}

	apiutil.Success(ctx, http.StatusOK, resp)
}

func (h *Handler) AbortMultipartUpload(ctx *gin.Context) {
	var uri jobIDURI
	if err := ctx.ShouldBindUri(&uri); err != nil {
		apiutil.Error(ctx, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.uploadService.AbortUpload(ctx.Request.Context(), uri.ID); err != nil {
		switch {
		case errors.Is(err, upload.ErrJobNotFound):
			apiutil.Error(ctx, http.StatusNotFound, "upload job not found")
		case errors.Is(err, upload.ErrJobAlreadyCompleted):
			apiutil.Error(ctx, http.StatusConflict, "upload job is already completed")
		default:
			slog.ErrorContext(
				ctx.Request.Context(),
				"abort upload failed",
				"job_id",
				uri.ID,
				"error",
				err,
			)
			apiutil.InternalError(ctx)
		}
		return
	}

	apiutil.Success(ctx, http.StatusOK, gin.H{"message": "upload aborted"})
}

func (h *Handler) ReportPartUploaded(ctx *gin.Context) {
	var uri jobIDURI
	if err := ctx.ShouldBindUri(&uri); err != nil {
		apiutil.Error(ctx, http.StatusBadRequest, err.Error())
		return
	}

	var body upload.ReportPartUploadedRequest
	if err := ctx.ShouldBindJSON(&body); err != nil {
		apiutil.Error(ctx, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.uploadService.ReportPartUploaded(ctx.Request.Context(), uri.ID, body); err != nil {
		switch {
		case errors.Is(err, upload.ErrJobNotFound):
			apiutil.Error(ctx, http.StatusNotFound, "upload job not found")
		case errors.Is(err, upload.ErrJobAlreadyCompleted):
			apiutil.Error(ctx, http.StatusConflict, "upload job is already completed")
		case errors.Is(err, upload.ErrJobAborted):
			apiutil.Error(ctx, http.StatusConflict, "upload job has been aborted")
		default:
			slog.ErrorContext(
				ctx.Request.Context(),
				"report part uploaded failed",
				"job_id",
				uri.ID,
				"error",
				err,
			)
			apiutil.InternalError(ctx)
		}
		return
	}

	apiutil.Success(
		ctx,
		http.StatusOK,
		gin.H{"part_number": body.PartNumber, "status": "completed"},
	)
}

func (h *Handler) GetUploadStatus(ctx *gin.Context) {
	var uri jobIDURI
	if err := ctx.ShouldBindUri(&uri); err != nil {
		apiutil.Error(ctx, http.StatusBadRequest, err.Error())
		return
	}

	job, err := h.uploadService.GetUploadStatus(ctx.Request.Context(), uri.ID)
	if err != nil {
		if errors.Is(err, upload.ErrJobNotFound) {
			apiutil.Error(ctx, http.StatusNotFound, "upload job not found")
			return
		}
		slog.ErrorContext(
			ctx.Request.Context(),
			"get upload status failed",
			"job_id",
			uri.ID,
			"error",
			err,
		)
		apiutil.InternalError(ctx)
		return
	}

	apiutil.Success(ctx, http.StatusOK, job)
}
