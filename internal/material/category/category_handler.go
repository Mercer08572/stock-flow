package category

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Mercer08572/stock-flow/pkg/response"
)

type CategoryHandler interface {
	RegisterRoutes(router gin.IRouter)
}

type categoryHandler struct {
	service CategoryService
}

type createCategoryRequest struct {
	Code     string  `json:"code"`
	Name     string  `json:"name"`
	ParentID *int64  `json:"parent_id"`
	Status   Status  `json:"status"`
	Remark   *string `json:"remark"`
}

type updateCategoryRequest struct {
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

func (h *categoryHandler) List(c *gin.Context) {
	filter, err := parseCategoryListFilter(c)
	if err != nil {
		writeCategoryError(c, err)
		return
	}

	result, err := h.service.List(c.Request.Context(), filter)
	if err != nil {
		writeCategoryError(c, err)
		return
	}

	response.Success(c, result)
}

func (h *categoryHandler) Get(c *gin.Context) {
	id, err := parseCategoryID(c)
	if err != nil {
		writeCategoryError(c, err)
		return
	}

	category, err := h.service.Get(c.Request.Context(), id)
	if err != nil {
		writeCategoryError(c, err)
		return
	}

	response.Success(c, category)
}

func (h *categoryHandler) Create(c *gin.Context) {
	var req createCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeCategoryError(c, NewValidationError("request body must be valid JSON"))
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
		writeCategoryError(c, err)
		return
	}

	response.Created(c, category)
}

func (h *categoryHandler) Update(c *gin.Context) {
	id, err := parseCategoryID(c)
	if err != nil {
		writeCategoryError(c, err)
		return
	}

	var req updateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeCategoryError(c, NewValidationError("request body must be valid JSON"))
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
		writeCategoryError(c, err)
		return
	}

	response.Success(c, category)
}

func (h *categoryHandler) Delete(c *gin.Context) {
	id, err := parseCategoryID(c)
	if err != nil {
		writeCategoryError(c, err)
		return
	}

	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		writeCategoryError(c, err)
		return
	}

	response.NoContent(c)
}

func parseCategoryID(c *gin.Context) (int64, error) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, NewValidationError("material category id must be greater than zero")
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
			return CategoryListFilter{}, NewValidationError("parent_id must be an integer")
		}
		filter.ParentID = &parentID
	}

	if rawLimit := c.Query("limit"); rawLimit != "" {
		limit, err := strconv.ParseInt(rawLimit, 10, 32)
		if err != nil {
			return CategoryListFilter{}, NewValidationError("limit must be an integer")
		}
		filter.Limit = int32(limit)
	}

	if rawOffset := c.Query("offset"); rawOffset != "" {
		offset, err := strconv.ParseInt(rawOffset, 10, 32)
		if err != nil {
			return CategoryListFilter{}, NewValidationError("offset must be an integer")
		}
		filter.Offset = int32(offset)
	}

	return filter, nil
}

func writeCategoryError(c *gin.Context, err error) {
	switch {
	case IsValidationError(err), errors.Is(err, ErrParentNotFound):
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
	case errors.Is(err, ErrDuplicateCode):
		response.Error(c, http.StatusConflict, response.CodeConflict, err.Error())
	case errors.Is(err, ErrNotFound):
		response.Error(c, http.StatusNotFound, response.CodeNotFound, err.Error())
	default:
		response.Error(c, http.StatusInternalServerError, response.CodeInternalError, "internal server error")
	}
}
