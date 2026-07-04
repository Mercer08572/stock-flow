package conversion

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/Mercer08572/stock-flow/pkg/response"
)

type Handler interface {
	RegisterRoutes(router gin.IRouter)
}

type handler struct {
	service Service
}

type DecimalString struct {
	Value string `json:"-"`
}

type CreateConversionRequest struct {
	FromUnitID int64         `json:"from_unit_id"`
	ToUnitID   int64         `json:"to_unit_id"`
	Factor     DecimalString `json:"factor" swaggertype:"string" example:"1.000000"`
}

type UpdateConversionRequest struct {
	FromUnitID int64         `json:"from_unit_id"`
	ToUnitID   int64         `json:"to_unit_id"`
	Factor     DecimalString `json:"factor" swaggertype:"string" example:"1.000000"`
}

func NewHandler(service Service) Handler {
	return &handler{service: service}
}

func (h *handler) RegisterRoutes(router gin.IRouter) {
	conversions := router.Group("/materials/:id/unit-conversions")
	conversions.GET("", h.List)
	conversions.GET("/:conversion_id", h.Get)
	conversions.POST("", h.Create)
	conversions.PUT("/:conversion_id", h.Update)
	conversions.DELETE("/:conversion_id", h.Delete)
}

func (d *DecimalString) UnmarshalJSON(src []byte) error {
	trimmed := bytes.TrimSpace(src)
	if bytes.Equal(trimmed, []byte("null")) {
		return errors.New("factor is required")
	}

	if len(trimmed) > 0 && trimmed[0] == '"' {
		var value string
		if err := json.Unmarshal(trimmed, &value); err != nil {
			return err
		}
		d.Value = value
		return nil
	}

	d.Value = string(trimmed)
	return nil
}

// List godoc
// @Summary List material unit conversions
// @Tags Material Unit Conversions
// @Produce json
// @Param id path int true "Material ID"
// @Param from_unit_id query int false "Source unit ID"
// @Param to_unit_id query int false "Target unit ID"
// @Param limit query int false "Page size"
// @Param offset query int false "Page offset"
// @Success 200 {object} response.Body{data=ListResult}
// @Failure 400 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /materials/{id}/unit-conversions [get]
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
// @Summary Get a material unit conversion
// @Tags Material Unit Conversions
// @Produce json
// @Param id path int true "Material ID"
// @Param conversion_id path int true "Material unit conversion ID"
// @Success 200 {object} response.Body{data=MaterialUnitConversion}
// @Failure 400 {object} response.Body
// @Failure 404 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /materials/{id}/unit-conversions/{conversion_id} [get]
func (h *handler) Get(c *gin.Context) {
	materialID, conversionID, err := parseMaterialAndConversionID(c)
	if err != nil {
		writeError(c, err)
		return
	}

	conversion, err := h.service.Get(c.Request.Context(), materialID, conversionID)
	if err != nil {
		writeError(c, err)
		return
	}

	response.Success(c, conversion)
}

// Create godoc
// @Summary Create a material unit conversion
// @Tags Material Unit Conversions
// @Accept json
// @Produce json
// @Param id path int true "Material ID"
// @Param body body CreateConversionRequest true "Material unit conversion payload"
// @Success 201 {object} response.Body{data=MaterialUnitConversion}
// @Failure 400 {object} response.Body
// @Failure 409 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /materials/{id}/unit-conversions [post]
func (h *handler) Create(c *gin.Context) {
	materialID, err := parseMaterialID(c)
	if err != nil {
		writeError(c, err)
		return
	}

	var req CreateConversionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, NewValidationError("request body must be valid JSON"))
		return
	}

	conversion, err := h.service.Create(c.Request.Context(), CreateInput{
		MaterialID: materialID,
		FromUnitID: req.FromUnitID,
		ToUnitID:   req.ToUnitID,
		Factor:     req.Factor.Value,
	})
	if err != nil {
		writeError(c, err)
		return
	}

	response.Created(c, conversion)
}

// Update godoc
// @Summary Update a material unit conversion
// @Tags Material Unit Conversions
// @Accept json
// @Produce json
// @Param id path int true "Material ID"
// @Param conversion_id path int true "Material unit conversion ID"
// @Param body body UpdateConversionRequest true "Material unit conversion payload"
// @Success 200 {object} response.Body{data=MaterialUnitConversion}
// @Failure 400 {object} response.Body
// @Failure 404 {object} response.Body
// @Failure 409 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /materials/{id}/unit-conversions/{conversion_id} [put]
func (h *handler) Update(c *gin.Context) {
	materialID, conversionID, err := parseMaterialAndConversionID(c)
	if err != nil {
		writeError(c, err)
		return
	}

	var req UpdateConversionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, NewValidationError("request body must be valid JSON"))
		return
	}

	conversion, err := h.service.Update(c.Request.Context(), UpdateInput{
		ID:         conversionID,
		MaterialID: materialID,
		FromUnitID: req.FromUnitID,
		ToUnitID:   req.ToUnitID,
		Factor:     req.Factor.Value,
	})
	if err != nil {
		writeError(c, err)
		return
	}

	response.Success(c, conversion)
}

// Delete godoc
// @Summary Delete a material unit conversion
// @Tags Material Unit Conversions
// @Produce json
// @Param id path int true "Material ID"
// @Param conversion_id path int true "Material unit conversion ID"
// @Success 200 {object} response.Body
// @Failure 400 {object} response.Body
// @Failure 404 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /materials/{id}/unit-conversions/{conversion_id} [delete]
func (h *handler) Delete(c *gin.Context) {
	materialID, conversionID, err := parseMaterialAndConversionID(c)
	if err != nil {
		writeError(c, err)
		return
	}

	if err := h.service.Delete(c.Request.Context(), materialID, conversionID); err != nil {
		writeError(c, err)
		return
	}

	response.NoContent(c)
}

func parseListFilter(c *gin.Context) (ListFilter, error) {
	materialID, err := parseMaterialID(c)
	if err != nil {
		return ListFilter{}, err
	}

	filter := ListFilter{
		MaterialID: materialID,
		Limit:      DefaultListLimit,
		Offset:     0,
	}

	if rawFromUnitID := c.Query("from_unit_id"); rawFromUnitID != "" {
		fromUnitID, err := strconv.ParseInt(rawFromUnitID, 10, 64)
		if err != nil {
			return ListFilter{}, NewValidationError("from_unit_id must be an integer")
		}
		filter.FromUnitID = &fromUnitID
	}

	if rawToUnitID := c.Query("to_unit_id"); rawToUnitID != "" {
		toUnitID, err := strconv.ParseInt(rawToUnitID, 10, 64)
		if err != nil {
			return ListFilter{}, NewValidationError("to_unit_id must be an integer")
		}
		filter.ToUnitID = &toUnitID
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

func parseMaterialAndConversionID(c *gin.Context) (int64, int64, error) {
	materialID, err := parseMaterialID(c)
	if err != nil {
		return 0, 0, err
	}

	conversionID, err := strconv.ParseInt(c.Param("conversion_id"), 10, 64)
	if err != nil || conversionID <= 0 {
		return 0, 0, NewValidationError("material unit conversion id must be greater than zero")
	}

	return materialID, conversionID, nil
}

func parseMaterialID(c *gin.Context) (int64, error) {
	materialID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || materialID <= 0 {
		return 0, NewValidationError("material_id must be greater than zero")
	}

	return materialID, nil
}

func writeError(c *gin.Context, err error) {
	switch {
	case IsValidationError(err):
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
	case errors.Is(err, ErrDuplicatePair), errors.Is(err, ErrReversePair):
		response.Error(c, http.StatusConflict, response.CodeConflict, err.Error())
	case errors.Is(err, ErrMaterialNotFound), errors.Is(err, ErrFromUnitNotFound), errors.Is(err, ErrToUnitNotFound):
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
	case errors.Is(err, ErrNotFound):
		response.Error(c, http.StatusNotFound, response.CodeNotFound, err.Error())
	default:
		response.Error(c, http.StatusInternalServerError, response.CodeInternalError, "internal server error")
	}
}

func (d DecimalString) String() string {
	return strings.TrimSpace(d.Value)
}
