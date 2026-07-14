package inventory

import "context"

type ReferenceService interface {
	HasWarehouseReferences(ctx context.Context, warehouseID int64) (bool, error)
	HasSKUReferences(ctx context.Context, skuID int64) (bool, error)
}

type ReferenceRepository interface {
	HasWarehouseReferences(ctx context.Context, warehouseID int64) (bool, error)
	HasSKUReferences(ctx context.Context, skuID int64) (bool, error)
}

type referenceService struct {
	repo ReferenceRepository
}

func NewReferenceService(repo ReferenceRepository) ReferenceService {
	return &referenceService{repo: repo}
}

func (s *referenceService) HasWarehouseReferences(ctx context.Context, warehouseID int64) (bool, error) {
	return s.repo.HasWarehouseReferences(ctx, warehouseID)
}

func (s *referenceService) HasSKUReferences(ctx context.Context, skuID int64) (bool, error) {
	return s.repo.HasSKUReferences(ctx, skuID)
}
