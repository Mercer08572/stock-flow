package sku

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Mercer08572/stock-flow/pkg/response"
)

type Handler interface {
	RegisterRoutes(router gin.IRouter)
}

type handler struct {
	service Service
}

type createSKURequest struct {
	MaterialID int64   `json:"material_id"`
	Code       string  `json:"code"`
	Name       string  `json:"name"`
	UnitID     int64   `json:"unit_id"`
	Status     Status  `json:"status"`
	Remark     *string `json:"remark"`
}

type updateSKURequest struct {
	MaterialID int64   `json:"material_id"`
	Code       string  `json:"code"`
	Name       string  `json:"name"`
	UnitID     int64   `json:"unit_id"`
	Status     Status  `json:"status"`
	Remark     *string `json:"remark"`
}

func NewHandler(service Service) Handler {
	return &handler{service: service}
}

func (h *handler) RegisterRoutes(router gin.IRouter) {
	skus := router.Group("/skus")
	skus.GET("", h.List)
	skus.GET("/:id", h.Get)
	skus.POST("", h.Create)
	skus.PUT("/:id", h.Update)
	skus.DELETE("/:id", h.Delete)
}

func (h *handler) List(c *gin.Context) {
	filter, err := parseListFilter(c)
	if err != nil {
		writeError(c, err)
		return
	}

	result, err := h.service.List(c.Request.Context(), filter)
	if err != nil {
		writeError(c, err)
		return
	}

	response.Success(c, result)
}

func (h *handler) Get(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		writeError(c, err)
		return
	}

	item, err := h.service.Get(c.Request.Context(), id)
	if err != nil {
		writeError(c, err)
		return
	}

	response.Success(c, item)
}

func (h *handler) Create(c *gin.Context) {
	var req createSKURequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, NewValidationError("request body must be valid JSON"))
		return
	}

	item, err := h.service.Create(c.Request.Context(), CreateInput{
		MaterialID: req.MaterialID,
		Code:       req.Code,
		Name:       req.Name,
		UnitID:     req.UnitID,
		Status:     req.Status,
		Remark:     req.Remark,
	})
	if err != nil {
		writeError(c, err)
		return
	}

	response.Created(c, item)
}

func (h *handler) Update(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		writeError(c, err)
		return
	}

	var req updateSKURequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, NewValidationError("request body must be valid JSON"))
		return
	}

	item, err := h.service.Update(c.Request.Context(), UpdateInput{
		ID:         id,
		MaterialID: req.MaterialID,
		Code:       req.Code,
		Name:       req.Name,
		UnitID:     req.UnitID,
		Status:     req.Status,
		Remark:     req.Remark,
	})
	if err != nil {
		writeError(c, err)
		return
	}

	response.Success(c, item)
}

func (h *handler) Delete(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		writeError(c, err)
		return
	}

	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		writeError(c, err)
		return
	}

	response.NoContent(c)
}

func parseID(c *gin.Context) (int64, error) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, NewValidationError("sku id must be greater than zero")
	}

	return id, nil
}

func parseListFilter(c *gin.Context) (ListFilter, error) {
	filter := ListFilter{
		Limit:  DefaultListLimit,
		Offset: 0,
	}

	if rawStatus := c.Query("status"); rawStatus != "" {
		status := Status(rawStatus)
		filter.Status = &status
	}

	if rawMaterialID := c.Query("material_id"); rawMaterialID != "" {
		materialID, err := strconv.ParseInt(rawMaterialID, 10, 64)
		if err != nil {
			return ListFilter{}, NewValidationError("material_id must be an integer")
		}
		filter.MaterialID = &materialID
	}

	if rawUnitID := c.Query("unit_id"); rawUnitID != "" {
		unitID, err := strconv.ParseInt(rawUnitID, 10, 64)
		if err != nil {
			return ListFilter{}, NewValidationError("unit_id must be an integer")
		}
		filter.UnitID = &unitID
	}

	if rawLimit := c.Query("limit"); rawLimit != "" {
		limit, err := strconv.ParseInt(rawLimit, 10, 32)
		if err != nil {
			return ListFilter{}, NewValidationError("limit must be an integer")
		}
		filter.Limit = int32(limit)
	}

	if rawOffset := c.Query("offset"); rawOffset != "" {
		offset, err := strconv.ParseInt(rawOffset, 10, 32)
		if err != nil {
			return ListFilter{}, NewValidationError("offset must be an integer")
		}
		filter.Offset = int32(offset)
	}

	return filter, nil
}

func writeError(c *gin.Context, err error) {
	switch {
	case IsValidationError(err):
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
	case errors.Is(err, ErrDuplicateCode), errors.Is(err, ErrActiveSKUForMaterial):
		response.Error(c, http.StatusConflict, response.CodeConflict, err.Error())
	case errors.Is(err, ErrMaterialNotFound), errors.Is(err, ErrUnitNotFound), errors.Is(err, ErrInvalidUnit):
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
	case errors.Is(err, ErrNotFound):
		response.Error(c, http.StatusNotFound, response.CodeNotFound, err.Error())
	default:
		response.Error(c, http.StatusInternalServerError, response.CodeInternalError, "internal server error")
	}
}
