package inventory

import (
	"context"
	"errors"
	"strings"

	"github.com/Mercer08572/stock-flow/internal/shared/search"
	sku "github.com/Mercer08572/stock-flow/internal/sku"
	"github.com/Mercer08572/stock-flow/pkg/apperr"
)

// BatchService 是批次主数据的应用服务。
type BatchService interface {
	ListBatches(ctx context.Context, filter ListBatchesFilter) (BatchListResult, error)
	GetBatch(ctx context.Context, id int64) (*BatchDetail, error)
	CreateBatch(ctx context.Context, input CreateBatchInput) (*Batch, error)
	UpdateBatch(ctx context.Context, input UpdateBatchInput) (*Batch, error)
	DisableBatch(ctx context.Context, id int64) (*Batch, error)
	DeleteBatch(ctx context.Context, id int64) error
}

// BatchRepository 是批次主数据的持久化接口。
//
// 与库存操作使用的 Repository 分开：批次读写不需要库存余额、库存层或流水方法，
// 窄接口让服务层依赖更清晰，也让测试替身更小。
type BatchRepository interface {
	ListBatches(ctx context.Context, filter ListBatchesFilter) ([]Batch, error)
	CountBatches(ctx context.Context, filter ListBatchesFilter) (int64, error)
	GetBatchByID(ctx context.Context, id int64) (*Batch, error)
	CreateBatch(ctx context.Context, input CreateBatchRecord) (int64, error)
	UpdateBatch(ctx context.Context, input UpdateBatchRecord) error
	DisableBatch(ctx context.Context, id int64) (*Batch, error)
	DeleteBatch(ctx context.Context, id int64) error
	BatchNoExistsForSKU(ctx context.Context, skuID int64, batchNo string, excludeID *int64) (bool, error)
	CountBatchReferences(ctx context.Context, id int64) (int64, error)
	ListBatchWarehouseStocks(ctx context.Context, batchID int64) ([]BatchWarehouseStock, error)
}

type batchService struct {
	repo      BatchRepository
	skuReader SKUReader
}

// NewBatchService 通过构造函数注入依赖。
func NewBatchService(repo BatchRepository, skuReader SKUReader) BatchService {
	return &batchService{repo: repo, skuReader: skuReader}
}

func (s *batchService) ListBatches(ctx context.Context, filter ListBatchesFilter) (BatchListResult, error) {
	normalized, err := normalizeListBatchesFilter(filter)
	if err != nil {
		return BatchListResult{}, err
	}

	total, err := s.repo.CountBatches(ctx, normalized)
	if err != nil {
		return BatchListResult{}, err
	}

	items, err := s.repo.ListBatches(ctx, normalized)
	if err != nil {
		return BatchListResult{}, err
	}

	return BatchListResult{
		Items:  items,
		Total:  total,
		Limit:  normalized.Limit,
		Offset: normalized.Offset,
	}, nil
}

func (s *batchService) GetBatch(ctx context.Context, id int64) (*BatchDetail, error) {
	if id <= 0 {
		return nil, apperr.NewValidationError("batch id must be greater than zero")
	}

	batch, err := s.repo.GetBatchByID(ctx, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrBatchNotFound
		}
		return nil, err
	}

	stocks, err := s.repo.ListBatchWarehouseStocks(ctx, id)
	if err != nil {
		return nil, err
	}

	return &BatchDetail{Batch: *batch, Stocks: stocks}, nil
}

func (s *batchService) CreateBatch(ctx context.Context, input CreateBatchInput) (*Batch, error) {
	normalized, err := s.normalizeCreateBatchInput(ctx, input)
	if err != nil {
		return nil, err
	}

	exists, err := s.repo.BatchNoExistsForSKU(ctx, normalized.SKUID, normalized.BatchNo, nil)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrBatchNoDuplicate
	}

	id, err := s.repo.CreateBatch(ctx, CreateBatchRecord{
		SKUID:            normalized.SKUID,
		BatchNo:          normalized.BatchNo,
		ProductionDate:   normalized.ProductionDate,
		ExpirationDate:   normalized.ExpirationDate,
		Status:           normalized.Status,
		DispatchPriority: *normalized.DispatchPriority,
		Remark:           normalized.Remark,
	})
	if err != nil {
		return nil, err
	}

	return s.getBatchOrNotFound(ctx, id)
}

func (s *batchService) UpdateBatch(ctx context.Context, input UpdateBatchInput) (*Batch, error) {
	if input.ID <= 0 {
		return nil, apperr.NewValidationError("batch id must be greater than zero")
	}

	existing, err := s.repo.GetBatchByID(ctx, input.ID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrBatchNotFound
		}
		return nil, err
	}

	productionDate, err := normalizeOptionalDate(input.ProductionDate)
	if err != nil {
		return nil, err
	}
	expirationDate, err := normalizeOptionalDate(input.ExpirationDate)
	if err != nil {
		return nil, err
	}
	if err := validateBatchDateRange(productionDate, expirationDate); err != nil {
		return nil, err
	}

	// status / dispatch_priority 省略时保留原值；效期与备注按全量替换处理（空值即清空）。
	status := existing.Status
	if input.Status != nil && strings.TrimSpace(string(*input.Status)) != "" {
		status, err = normalizeBatchStatus(*input.Status)
		if err != nil {
			return nil, err
		}
	}

	priority := existing.DispatchPriority
	if input.DispatchPriority != nil {
		priority, err = normalizeDispatchPriority(input.DispatchPriority)
		if err != nil {
			return nil, err
		}
	}

	if err := s.repo.UpdateBatch(ctx, UpdateBatchRecord{
		ID:               input.ID,
		ProductionDate:   productionDate,
		ExpirationDate:   expirationDate,
		Status:           status,
		DispatchPriority: priority,
		Remark:           normalizeRemark(input.Remark),
	}); err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrBatchNotFound
		}
		return nil, err
	}

	return s.getBatchOrNotFound(ctx, input.ID)
}

func (s *batchService) DisableBatch(ctx context.Context, id int64) (*Batch, error) {
	if id <= 0 {
		return nil, apperr.NewValidationError("batch id must be greater than zero")
	}

	batch, err := s.repo.DisableBatch(ctx, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrBatchNotFound
		}
		return nil, err
	}

	return batch, nil
}

func (s *batchService) DeleteBatch(ctx context.Context, id int64) error {
	if id <= 0 {
		return apperr.NewValidationError("batch id must be greater than zero")
	}

	if _, err := s.repo.GetBatchByID(ctx, id); err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrBatchNotFound
		}
		return err
	}

	references, err := s.repo.CountBatchReferences(ctx, id)
	if err != nil {
		return err
	}
	if references > 0 {
		return ErrBatchReferenced
	}

	if err := s.repo.DeleteBatch(ctx, id); err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrBatchNotFound
		}
		return err
	}
	return nil
}

func (s *batchService) getBatchOrNotFound(ctx context.Context, id int64) (*Batch, error) {
	batch, err := s.repo.GetBatchByID(ctx, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrBatchNotFound
		}
		return nil, err
	}
	return batch, nil
}

func (s *batchService) normalizeCreateBatchInput(ctx context.Context, input CreateBatchInput) (CreateBatchInput, error) {
	if input.SKUID <= 0 {
		return CreateBatchInput{}, apperr.NewValidationError("sku_id must be greater than zero")
	}

	batchNo, err := normalizeBatchNo(input.BatchNo)
	if err != nil {
		return CreateBatchInput{}, err
	}

	productionDate, err := normalizeOptionalDate(input.ProductionDate)
	if err != nil {
		return CreateBatchInput{}, err
	}
	expirationDate, err := normalizeOptionalDate(input.ExpirationDate)
	if err != nil {
		return CreateBatchInput{}, err
	}
	if err := validateBatchDateRange(productionDate, expirationDate); err != nil {
		return CreateBatchInput{}, err
	}

	status, err := normalizeBatchStatus(input.Status)
	if err != nil {
		return CreateBatchInput{}, err
	}

	priority, err := normalizeDispatchPriority(input.DispatchPriority)
	if err != nil {
		return CreateBatchInput{}, err
	}

	if err := s.validateSKU(ctx, input.SKUID); err != nil {
		return CreateBatchInput{}, err
	}

	return CreateBatchInput{
		SKUID:            input.SKUID,
		BatchNo:          batchNo,
		ProductionDate:   productionDate,
		ExpirationDate:   expirationDate,
		Status:           status,
		DispatchPriority: &priority,
		Remark:           normalizeRemark(input.Remark),
	}, nil
}

func (s *batchService) validateSKU(ctx context.Context, skuID int64) error {
	if _, err := s.skuReader.Get(ctx, skuID); err != nil {
		if errors.Is(err, sku.ErrNotFound) {
			return ErrSKUNotFound
		}
		if apperr.IsValidationError(err) {
			return apperr.NewValidationError(err.Error())
		}
		return err
	}
	return nil
}

func normalizeListBatchesFilter(filter ListBatchesFilter) (ListBatchesFilter, error) {
	if filter.SKUID != nil && *filter.SKUID <= 0 {
		return ListBatchesFilter{}, apperr.NewValidationError("sku_id must be greater than zero")
	}

	if filter.Status != nil {
		status := BatchStatus(strings.TrimSpace(string(*filter.Status)))
		if status == "" {
			filter.Status = nil
		} else {
			if !status.IsValid() {
				return ListBatchesFilter{}, apperr.NewValidationError("status must be active or inactive")
			}
			filter.Status = &status
		}
	}

	keyword, err := search.NormalizeKeyword(filter.Keyword)
	if err != nil {
		return ListBatchesFilter{}, err
	}
	filter.Keyword = keyword

	if filter.Limit <= 0 {
		filter.Limit = DefaultListLimit
	}
	if filter.Limit > MaxListLimit {
		filter.Limit = MaxListLimit
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}

	return filter, nil
}
