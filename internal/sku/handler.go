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

type CreateSKURequest struct {
	MaterialID int64   `json:"material_id"`
	Code       string  `json:"code"`
	Name       string  `json:"name"`
	UnitID     int64   `json:"unit_id"`
	Status     Status  `json:"status"`
	Remark     *string `json:"remark"`
}

type UpdateSKURequest struct {
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

// List godoc
// @Summary List SKUs
// @Tags SKUs
// @Produce json
// @Param status query string false "SKU status" Enums(active,inactive)
// @Param material_id query int false "Material ID"
// @Param unit_id query int false "Unit ID"
// @Param limit query int false "Page size"
// @Param offset query int false "Page offset"
// @Success 200 {object} response.Body{data=ListResult}
// @Failure 400 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /skus [get]
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

// Get godoc
// @Summary Get a SKU
// @Tags SKUs
// @Produce json
// @Param id path int true "SKU ID"
// @Success 200 {object} response.Body{data=SKU}
// @Failure 400 {object} response.Body
// @Failure 404 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /skus/{id} [get]
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

// Create godoc
// @Summary Create a SKU
// @Tags SKUs
// @Accept json
// @Produce json
// @Param body body CreateSKURequest true "SKU payload"
// @Success 201 {object} response.Body{data=SKU}
// @Failure 400 {object} response.Body
// @Failure 409 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /skus [post]
func (h *handler) Create(c *gin.Context) {
	var req CreateSKURequest
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

// Update godoc
// @Summary Update a SKU
// @Tags SKUs
// @Accept json
// @Produce json
// @Param id path int true "SKU ID"
// @Param body body UpdateSKURequest true "SKU payload"
// @Success 200 {object} response.Body{data=SKU}
// @Failure 400 {object} response.Body
// @Failure 404 {object} response.Body
// @Failure 409 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /skus/{id} [put]
func (h *handler) Update(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		writeError(c, err)
		return
	}

	var req UpdateSKURequest
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

// Delete godoc
// @Summary Delete a SKU
// @Tags SKUs
// @Produce json
// @Param id path int true "SKU ID"
// @Success 200 {object} response.Body
// @Failure 400 {object} response.Body
// @Failure 404 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /skus/{id} [delete]
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
