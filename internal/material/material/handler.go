package material

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Mercer08572/stock-flow/pkg/apperr"
	"github.com/Mercer08572/stock-flow/pkg/response"

	"github.com/Mercer08572/stock-flow/internal/shared/httperr"
)

type Handler interface {
	RegisterRoutes(router gin.IRouter)
}

type handler struct {
	service Service
}

type CreateMaterialRequest struct {
	Code       string  `json:"code"`
	Name       string  `json:"name"`
	CategoryID int64   `json:"category_id"`
	BaseUnitID int64   `json:"base_unit_id"`
	Status     Status  `json:"status"`
	Remark     *string `json:"remark"`
}

type UpdateMaterialRequest struct {
	Code       string  `json:"code"`
	Name       string  `json:"name"`
	CategoryID int64   `json:"category_id"`
	BaseUnitID int64   `json:"base_unit_id"`
	Status     Status  `json:"status"`
	Remark     *string `json:"remark"`
}

func NewHandler(service Service) Handler {
	return &handler{service: service}
}

func (h *handler) RegisterRoutes(router gin.IRouter) {
	materials := router.Group("/materials")
	materials.GET("", h.List)
	materials.GET("/:id", h.Get)
	materials.POST("", h.Create)
	materials.PUT("/:id", h.Update)
	materials.DELETE("/:id", h.Delete)
}

// List godoc
// @Summary List materials
// @Tags Materials
// @Produce json
// @Param status query string false "Material status" Enums(active,inactive)
// @Param category_id query int false "Material category ID"
// @Param limit query int false "Page size"
// @Param offset query int false "Page offset"
// @Success 200 {object} response.Body{data=ListResult}
// @Failure 400 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /materials [get]
func (h *handler) List(c *gin.Context) {
	filter, err := parseListFilter(c)
	if err != nil {
		httperr.Write(c, err)
		return
	}

	result, err := h.service.List(c.Request.Context(), filter)
	if err != nil {
		httperr.Write(c, err)
		return
	}

	response.Success(c, result)
}

// Get godoc
// @Summary Get a material
// @Tags Materials
// @Produce json
// @Param id path int true "Material ID"
// @Success 200 {object} response.Body{data=Material}
// @Failure 400 {object} response.Body
// @Failure 404 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /materials/{id} [get]
func (h *handler) Get(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		httperr.Write(c, err)
		return
	}

	material, err := h.service.Get(c.Request.Context(), id)
	if err != nil {
		httperr.Write(c, err)
		return
	}

	response.Success(c, material)
}

// Create godoc
// @Summary Create a material
// @Tags Materials
// @Accept json
// @Produce json
// @Param body body CreateMaterialRequest true "Material payload"
// @Success 201 {object} response.Body{data=Material}
// @Failure 400 {object} response.Body
// @Failure 409 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /materials [post]
func (h *handler) Create(c *gin.Context) {
	var req CreateMaterialRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.Write(c, apperr.NewValidationError("request body must be valid JSON"))
		return
	}

	material, err := h.service.Create(c.Request.Context(), CreateInput{
		Code:       req.Code,
		Name:       req.Name,
		CategoryID: req.CategoryID,
		BaseUnitID: req.BaseUnitID,
		Status:     req.Status,
		Remark:     req.Remark,
	})
	if err != nil {
		httperr.Write(c, err)
		return
	}

	response.Created(c, material)
}

// Update godoc
// @Summary Update a material
// @Tags Materials
// @Accept json
// @Produce json
// @Param id path int true "Material ID"
// @Param body body UpdateMaterialRequest true "Material payload"
// @Success 200 {object} response.Body{data=Material}
// @Failure 400 {object} response.Body
// @Failure 404 {object} response.Body
// @Failure 409 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /materials/{id} [put]
func (h *handler) Update(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		httperr.Write(c, err)
		return
	}

	var req UpdateMaterialRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.Write(c, apperr.NewValidationError("request body must be valid JSON"))
		return
	}

	material, err := h.service.Update(c.Request.Context(), UpdateInput{
		ID:         id,
		Code:       req.Code,
		Name:       req.Name,
		CategoryID: req.CategoryID,
		BaseUnitID: req.BaseUnitID,
		Status:     req.Status,
		Remark:     req.Remark,
	})
	if err != nil {
		httperr.Write(c, err)
		return
	}

	response.Success(c, material)
}

// Delete godoc
// @Summary Delete a material
// @Tags Materials
// @Produce json
// @Param id path int true "Material ID"
// @Success 200 {object} response.Body
// @Failure 400 {object} response.Body
// @Failure 404 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /materials/{id} [delete]
func (h *handler) Delete(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		httperr.Write(c, err)
		return
	}

	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		httperr.Write(c, err)
		return
	}

	response.NoContent(c)
}

func parseID(c *gin.Context) (int64, error) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, apperr.NewValidationError("material id must be greater than zero")
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

	if rawCategoryID := c.Query("category_id"); rawCategoryID != "" {
		categoryID, err := strconv.ParseInt(rawCategoryID, 10, 64)
		if err != nil {
			return ListFilter{}, apperr.NewValidationError("category_id must be an integer")
		}
		filter.CategoryID = &categoryID
	}

	if rawLimit := c.Query("limit"); rawLimit != "" {
		limit, err := strconv.ParseInt(rawLimit, 10, 32)
		if err != nil {
			return ListFilter{}, apperr.NewValidationError("limit must be an integer")
		}
		filter.Limit = int32(limit)
	}

	if rawOffset := c.Query("offset"); rawOffset != "" {
		offset, err := strconv.ParseInt(rawOffset, 10, 32)
		if err != nil {
			return ListFilter{}, apperr.NewValidationError("offset must be an integer")
		}
		filter.Offset = int32(offset)
	}

	return filter, nil
}
