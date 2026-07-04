package unit

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Mercer08572/stock-flow/pkg/response"
)

type UnitHandler interface {
	RegisterRoutes(router gin.IRouter)
}

type unitHandler struct {
	service UnitService
}

type CreateUnitRequest struct {
	Code      string   `json:"code"`
	Name      string   `json:"name"`
	Symbol    string   `json:"symbol"`
	UnitType  UnitType `json:"unit_type"`
	Precision int32    `json:"precision"`
	Status    Status   `json:"status"`
}

type UpdateUnitRequest struct {
	Code      string   `json:"code"`
	Name      string   `json:"name"`
	Symbol    string   `json:"symbol"`
	UnitType  UnitType `json:"unit_type"`
	Precision int32    `json:"precision"`
	Status    Status   `json:"status"`
}

func NewUnitHandler(service UnitService) UnitHandler {
	return &unitHandler{service: service}
}

func (h *unitHandler) RegisterRoutes(router gin.IRouter) {
	units := router.Group("/units")
	units.GET("", h.List)
	units.GET("/:id", h.Get)
	units.POST("", h.Create)
	units.PUT("/:id", h.Update)
	units.DELETE("/:id", h.Delete)
}

// List godoc
// @Summary List units
// @Tags Units
// @Produce json
// @Param status query string false "Unit status" Enums(active,inactive)
// @Param unit_type query string false "Unit type" Enums(count,weight,length,area,volume,package,time,other)
// @Param limit query int false "Page size"
// @Param offset query int false "Page offset"
// @Success 200 {object} response.Body{data=UnitListResult}
// @Failure 400 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /units [get]
func (h *unitHandler) List(c *gin.Context) {
	filter, err := parseUnitListFilter(c)
	if err != nil {
		writeUnitError(c, err)
		return
	}

	result, err := h.service.List(c.Request.Context(), filter)
	if err != nil {
		writeUnitError(c, err)
		return
	}

	response.Success(c, result)
}

// Get godoc
// @Summary Get a unit
// @Tags Units
// @Produce json
// @Param id path int true "Unit ID"
// @Success 200 {object} response.Body{data=Unit}
// @Failure 400 {object} response.Body
// @Failure 404 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /units/{id} [get]
func (h *unitHandler) Get(c *gin.Context) {
	id, err := parseUnitID(c)
	if err != nil {
		writeUnitError(c, err)
		return
	}

	unit, err := h.service.Get(c.Request.Context(), id)
	if err != nil {
		writeUnitError(c, err)
		return
	}

	response.Success(c, unit)
}

// Create godoc
// @Summary Create a unit
// @Tags Units
// @Accept json
// @Produce json
// @Param body body CreateUnitRequest true "Unit payload"
// @Success 201 {object} response.Body{data=Unit}
// @Failure 400 {object} response.Body
// @Failure 409 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /units [post]
func (h *unitHandler) Create(c *gin.Context) {
	var req CreateUnitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeUnitError(c, NewValidationError("request body must be valid JSON"))
		return
	}

	unit, err := h.service.Create(c.Request.Context(), CreateUnitInput{
		Code:      req.Code,
		Name:      req.Name,
		Symbol:    req.Symbol,
		UnitType:  req.UnitType,
		Precision: req.Precision,
		Status:    req.Status,
	})
	if err != nil {
		writeUnitError(c, err)
		return
	}

	response.Created(c, unit)
}

// Update godoc
// @Summary Update a unit
// @Tags Units
// @Accept json
// @Produce json
// @Param id path int true "Unit ID"
// @Param body body UpdateUnitRequest true "Unit payload"
// @Success 200 {object} response.Body{data=Unit}
// @Failure 400 {object} response.Body
// @Failure 404 {object} response.Body
// @Failure 409 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /units/{id} [put]
func (h *unitHandler) Update(c *gin.Context) {
	id, err := parseUnitID(c)
	if err != nil {
		writeUnitError(c, err)
		return
	}

	var req UpdateUnitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeUnitError(c, NewValidationError("request body must be valid JSON"))
		return
	}

	unit, err := h.service.Update(c.Request.Context(), UpdateUnitInput{
		ID:        id,
		Code:      req.Code,
		Name:      req.Name,
		Symbol:    req.Symbol,
		UnitType:  req.UnitType,
		Precision: req.Precision,
		Status:    req.Status,
	})
	if err != nil {
		writeUnitError(c, err)
		return
	}

	response.Success(c, unit)
}

// Delete godoc
// @Summary Delete a unit
// @Tags Units
// @Produce json
// @Param id path int true "Unit ID"
// @Success 200 {object} response.Body
// @Failure 400 {object} response.Body
// @Failure 404 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /units/{id} [delete]
func (h *unitHandler) Delete(c *gin.Context) {
	id, err := parseUnitID(c)
	if err != nil {
		writeUnitError(c, err)
		return
	}

	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		writeUnitError(c, err)
		return
	}

	response.NoContent(c)
}

func parseUnitID(c *gin.Context) (int64, error) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, NewValidationError("unit id must be greater than zero")
	}

	return id, nil
}

func parseUnitListFilter(c *gin.Context) (UnitListFilter, error) {
	filter := UnitListFilter{
		Limit:  DefaultListLimit,
		Offset: 0,
	}

	if rawStatus := c.Query("status"); rawStatus != "" {
		status := Status(rawStatus)
		filter.Status = &status
	}

	if rawUnitType := c.Query("unit_type"); rawUnitType != "" {
		unitType := UnitType(rawUnitType)
		filter.UnitType = &unitType
	}

	if rawLimit := c.Query("limit"); rawLimit != "" {
		limit, err := strconv.ParseInt(rawLimit, 10, 32)
		if err != nil {
			return UnitListFilter{}, NewValidationError("limit must be an integer")
		}
		filter.Limit = int32(limit)
	}

	if rawOffset := c.Query("offset"); rawOffset != "" {
		offset, err := strconv.ParseInt(rawOffset, 10, 32)
		if err != nil {
			return UnitListFilter{}, NewValidationError("offset must be an integer")
		}
		filter.Offset = int32(offset)
	}

	return filter, nil
}

func writeUnitError(c *gin.Context, err error) {
	switch {
	case IsValidationError(err):
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
	case errors.Is(err, ErrDuplicateCode):
		response.Error(c, http.StatusConflict, response.CodeConflict, err.Error())
	case errors.Is(err, ErrNotFound):
		response.Error(c, http.StatusNotFound, response.CodeNotFound, err.Error())
	default:
		response.Error(c, http.StatusInternalServerError, response.CodeInternalError, "internal server error")
	}
}
