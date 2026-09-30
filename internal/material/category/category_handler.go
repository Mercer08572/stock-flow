package category

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Mercer08572/stock-flow/pkg/apperr"
	"github.com/Mercer08572/stock-flow/pkg/response"

	"github.com/Mercer08572/stock-flow/internal/shared/httperr"
)

type CategoryHandler interface {
	RegisterRoutes(router gin.IRouter)
}

type categoryHandler struct {
	service CategoryService
}

type CreateCategoryRequest struct {
	Code     string  `json:"code"`
	Name     string  `json:"name"`
	ParentID *int64  `json:"parent_id"`
	Status   Status  `json:"status"`
	Remark   *string `json:"remark"`
}

type UpdateCategoryRequest struct {
	Code     string  `json:"code"`
	Name     string  `json:"name"`
	ParentID *int64  `json:"parent_id"`
	Status   Status  `json:"status"`
	Remark   *string `json:"remark"`
}

func NewCategoryHandler(service CategoryService) CategoryHandler {
	return &categoryHandler{service: service}
}

func (h *categoryHandler) RegisterRoutes(router gin.IRouter) {
	categories := router.Group("/material-categories")
	categories.GET("", h.List)
	categories.GET("/:id", h.Get)
	categories.POST("", h.Create)
	categories.PUT("/:id", h.Update)
	categories.DELETE("/:id", h.Delete)
}

// List godoc
// @Summary List material categories
// @Tags Material Categories
// @Produce json
// @Param status query string false "Category status" Enums(active,inactive)
// @Param parent_id query int false "Parent category ID"
// @Param limit query int false "Page size"
// @Param offset query int false "Page offset"
// @Success 200 {object} response.Body{data=CategoryListResult}
// @Failure 400 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /material-categories [get]
func (h *categoryHandler) List(c *gin.Context) {
	filter, err := parseCategoryListFilter(c)
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
// @Summary Get a material category
// @Tags Material Categories
// @Produce json
// @Param id path int true "Material category ID"
// @Success 200 {object} response.Body{data=Category}
// @Failure 400 {object} response.Body
// @Failure 404 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /material-categories/{id} [get]
func (h *categoryHandler) Get(c *gin.Context) {
	id, err := parseCategoryID(c)
	if err != nil {
		httperr.Write(c, err)
		return
	}

	category, err := h.service.Get(c.Request.Context(), id)
	if err != nil {
		httperr.Write(c, err)
		return
	}

	response.Success(c, category)
}

// Create godoc
// @Summary Create a material category
// @Tags Material Categories
// @Accept json
// @Produce json
// @Param body body CreateCategoryRequest true "Material category payload"
// @Success 201 {object} response.Body{data=Category}
// @Failure 400 {object} response.Body
// @Failure 409 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /material-categories [post]
func (h *categoryHandler) Create(c *gin.Context) {
	var req CreateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.Write(c, apperr.NewValidationError("request body must be valid JSON"))
		return
	}

	category, err := h.service.Create(c.Request.Context(), CreateCategoryInput{
		Code:     req.Code,
		Name:     req.Name,
		ParentID: req.ParentID,
		Status:   req.Status,
		Remark:   req.Remark,
	})
	if err != nil {
		httperr.Write(c, err)
		return
	}

	response.Created(c, category)
}

// Update godoc
// @Summary Update a material category
// @Tags Material Categories
// @Accept json
// @Produce json
// @Param id path int true "Material category ID"
// @Param body body UpdateCategoryRequest true "Material category payload"
// @Success 200 {object} response.Body{data=Category}
// @Failure 400 {object} response.Body
// @Failure 404 {object} response.Body
// @Failure 409 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /material-categories/{id} [put]
func (h *categoryHandler) Update(c *gin.Context) {
	id, err := parseCategoryID(c)
	if err != nil {
		httperr.Write(c, err)
		return
	}

	var req UpdateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.Write(c, apperr.NewValidationError("request body must be valid JSON"))
		return
	}

	category, err := h.service.Update(c.Request.Context(), UpdateCategoryInput{
		ID:       id,
		Code:     req.Code,
		Name:     req.Name,
		ParentID: req.ParentID,
		Status:   req.Status,
		Remark:   req.Remark,
	})
	if err != nil {
		httperr.Write(c, err)
		return
	}

	response.Success(c, category)
}

// Delete godoc
// @Summary Delete a material category
// @Tags Material Categories
// @Produce json
// @Param id path int true "Material category ID"
// @Success 200 {object} response.Body
// @Failure 400 {object} response.Body
// @Failure 404 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /material-categories/{id} [delete]
func (h *categoryHandler) Delete(c *gin.Context) {
	id, err := parseCategoryID(c)
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

func parseCategoryID(c *gin.Context) (int64, error) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, apperr.NewValidationError("material category id must be greater than zero")
	}

	return id, nil
}

func parseCategoryListFilter(c *gin.Context) (CategoryListFilter, error) {
	filter := CategoryListFilter{
		Limit:  DefaultListLimit,
		Offset: 0,
	}

	if rawStatus := c.Query("status"); rawStatus != "" {
		status := Status(rawStatus)
		filter.Status = &status
	}

	if rawParentID := c.Query("parent_id"); rawParentID != "" {
		parentID, err := strconv.ParseInt(rawParentID, 10, 64)
		if err != nil {
			return CategoryListFilter{}, apperr.NewValidationError("parent_id must be an integer")
		}
		filter.ParentID = &parentID
	}

	if rawLimit := c.Query("limit"); rawLimit != "" {
		limit, err := strconv.ParseInt(rawLimit, 10, 32)
		if err != nil {
			return CategoryListFilter{}, apperr.NewValidationError("limit must be an integer")
		}
		filter.Limit = int32(limit)
	}

	if rawOffset := c.Query("offset"); rawOffset != "" {
		offset, err := strconv.ParseInt(rawOffset, 10, 32)
		if err != nil {
			return CategoryListFilter{}, apperr.NewValidationError("offset must be an integer")
		}
		filter.Offset = int32(offset)
	}

	return filter, nil
}
