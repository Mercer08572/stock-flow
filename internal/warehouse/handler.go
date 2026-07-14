package warehouse

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

type CreateWarehouseRequest struct {
	Code         string  `json:"code"`
	Name         string  `json:"name"`
	Type         Type    `json:"type"`
	Status       Status  `json:"status"`
	Location     *string `json:"location"`
	ContactName  *string `json:"contact_name"`
	ContactPhone *string `json:"contact_phone"`
	Remark       *string `json:"remark"`
}

type UpdateWarehouseRequest struct {
	Code         string  `json:"code"`
	Name         string  `json:"name"`
	Type         Type    `json:"type"`
	Status       Status  `json:"status"`
	Location     *string `json:"location"`
	ContactName  *string `json:"contact_name"`
	ContactPhone *string `json:"contact_phone"`
	Remark       *string `json:"remark"`
}

func NewHandler(service Service) Handler {
	return &handler{service: service}
}

func (h *handler) RegisterRoutes(router gin.IRouter) {
	warehouses := router.Group("/warehouses")
	warehouses.GET("", h.List)
	warehouses.GET("/:id", h.Get)
	warehouses.POST("", h.Create)
	warehouses.PUT("/:id", h.Update)
	warehouses.DELETE("/:id", h.Delete)
	warehouses.PUT("/:id/disable", h.Disable)
}

// List godoc
// @Summary List warehouses
// @Tags Warehouses
// @Produce json
// @Param status query string false "Warehouse status" Enums(active,inactive)
// @Param type query string false "Warehouse type" Enums(normal,virtual)
// @Param limit query int false "Page size"
// @Param offset query int false "Page offset"
// @Success 200 {object} response.Body{data=ListResult}
// @Failure 400 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /warehouses [get]
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
// @Summary Get a warehouse
// @Tags Warehouses
// @Produce json
// @Param id path int true "Warehouse ID"
// @Success 200 {object} response.Body{data=Warehouse}
// @Failure 400 {object} response.Body
// @Failure 404 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /warehouses/{id} [get]
func (h *handler) Get(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		writeError(c, err)
		return
	}

	warehouse, err := h.service.Get(c.Request.Context(), id)
	if err != nil {
		writeError(c, err)
		return
	}

	response.Success(c, warehouse)
}

// Create godoc
// @Summary Create a warehouse
// @Tags Warehouses
// @Accept json
// @Produce json
// @Param body body CreateWarehouseRequest true "Warehouse payload"
// @Success 201 {object} response.Body{data=Warehouse}
// @Failure 400 {object} response.Body
// @Failure 409 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /warehouses [post]
func (h *handler) Create(c *gin.Context) {
	var req CreateWarehouseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, NewValidationError("request body must be valid JSON"))
		return
	}

	warehouse, err := h.service.Create(c.Request.Context(), CreateInput{
		Code:         req.Code,
		Name:         req.Name,
		Type:         req.Type,
		Status:       req.Status,
		Location:     req.Location,
		ContactName:  req.ContactName,
		ContactPhone: req.ContactPhone,
		Remark:       req.Remark,
	})
	if err != nil {
		writeError(c, err)
		return
	}

	response.Created(c, warehouse)
}

// Update godoc
// @Summary Update a warehouse
// @Tags Warehouses
// @Accept json
// @Produce json
// @Param id path int true "Warehouse ID"
// @Param body body UpdateWarehouseRequest true "Warehouse payload"
// @Success 200 {object} response.Body{data=Warehouse}
// @Failure 400 {object} response.Body
// @Failure 404 {object} response.Body
// @Failure 409 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /warehouses/{id} [put]
func (h *handler) Update(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		writeError(c, err)
		return
	}

	var req UpdateWarehouseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, NewValidationError("request body must be valid JSON"))
		return
	}

	warehouse, err := h.service.Update(c.Request.Context(), UpdateInput{
		ID:           id,
		Code:         req.Code,
		Name:         req.Name,
		Type:         req.Type,
		Status:       req.Status,
		Location:     req.Location,
		ContactName:  req.ContactName,
		ContactPhone: req.ContactPhone,
		Remark:       req.Remark,
	})
	if err != nil {
		writeError(c, err)
		return
	}

	response.Success(c, warehouse)
}

// Delete godoc
// @Summary Delete a warehouse
// @Tags Warehouses
// @Produce json
// @Param id path int true "Warehouse ID"
// @Success 200 {object} response.Body
// @Failure 400 {object} response.Body
// @Failure 404 {object} response.Body
// @Failure 409 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /warehouses/{id} [delete]
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

// Disable godoc
// @Summary Disable a warehouse
// @Tags Warehouses
// @Produce json
// @Param id path int true "Warehouse ID"
// @Success 200 {object} response.Body{data=Warehouse}
// @Failure 400 {object} response.Body
// @Failure 404 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /warehouses/{id}/disable [put]
func (h *handler) Disable(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		writeError(c, err)
		return
	}

	warehouse, err := h.service.Disable(c.Request.Context(), id)
	if err != nil {
		writeError(c, err)
		return
	}

	response.Success(c, warehouse)
}

func parseID(c *gin.Context) (int64, error) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, NewValidationError("warehouse id must be greater than zero")
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

	if rawType := c.Query("type"); rawType != "" {
		warehouseType := Type(rawType)
		filter.Type = &warehouseType
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
	case errors.Is(err, ErrDuplicateCode), errors.Is(err, ErrReferencedByInventory):
		response.Error(c, http.StatusConflict, response.CodeConflict, err.Error())
	case errors.Is(err, ErrNotFound):
		response.Error(c, http.StatusNotFound, response.CodeNotFound, err.Error())
	default:
		response.Error(c, http.StatusInternalServerError, response.CodeInternalError, "internal server error")
	}
}
