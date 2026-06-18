package material

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

type createMaterialRequest struct {
	Code       string  `json:"code"`
	Name       string  `json:"name"`
	CategoryID int64   `json:"category_id"`
	BaseUnitID int64   `json:"base_unit_id"`
	Status     Status  `json:"status"`
	Remark     *string `json:"remark"`
}

type updateMaterialRequest struct {
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

	material, err := h.service.Get(c.Request.Context(), id)
	if err != nil {
		writeError(c, err)
		return
	}

	response.Success(c, material)
}

func (h *handler) Create(c *gin.Context) {
	var req createMaterialRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, NewValidationError("request body must be valid JSON"))
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
		writeError(c, err)
		return
	}

	response.Created(c, material)
}

func (h *handler) Update(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		writeError(c, err)
		return
	}

	var req updateMaterialRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, NewValidationError("request body must be valid JSON"))
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
		writeError(c, err)
		return
	}

	response.Success(c, material)
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
		return 0, NewValidationError("material id must be greater than zero")
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
			return ListFilter{}, NewValidationError("category_id must be an integer")
		}
		filter.CategoryID = &categoryID
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
	case IsValidationError(err), errors.Is(err, ErrCategoryNotFound), errors.Is(err, ErrBaseUnitNotFound):
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
	case errors.Is(err, ErrDuplicateCode):
		response.Error(c, http.StatusConflict, response.CodeConflict, err.Error())
	case errors.Is(err, ErrNotFound):
		response.Error(c, http.StatusNotFound, response.CodeNotFound, err.Error())
	default:
		response.Error(c, http.StatusInternalServerError, response.CodeInternalError, "internal server error")
	}
}
