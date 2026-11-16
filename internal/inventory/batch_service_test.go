package inventory_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Mercer08572/stock-flow/internal/inventory"
	"github.com/Mercer08572/stock-flow/pkg/apperr"
)

type fakeBatchRepository struct {
	batches []inventory.Batch
	total   int64

	getBatch  *inventory.Batch
	getErr    error
	stocks    []inventory.BatchWarehouseStock
	refs      int64
	duplicate bool

	createID     int64
	createErr    error
	createRecord *inventory.CreateBatchRecord
	updateErr    error
	updateRecord *inventory.UpdateBatchRecord
	disableErr   error
	deleteErr    error
	deleteCalls  int
}

func (r *fakeBatchRepository) ListBatches(context.Context, inventory.ListBatchesFilter) ([]inventory.Batch, error) {
	return r.batches, nil
}

func (r *fakeBatchRepository) CountBatches(context.Context, inventory.ListBatchesFilter) (int64, error) {
	return r.total, nil
}

func (r *fakeBatchRepository) GetBatchByID(context.Context, int64) (*inventory.Batch, error) {
	if r.getErr != nil {
		return nil, r.getErr
	}
	if r.getBatch == nil {
		return nil, inventory.ErrNotFound
	}
	return r.getBatch, nil
}

func (r *fakeBatchRepository) CreateBatch(_ context.Context, input inventory.CreateBatchRecord) (int64, error) {
	if r.createErr != nil {
		return 0, r.createErr
	}
	record := input
	r.createRecord = &record
	return r.createID, nil
}

func (r *fakeBatchRepository) UpdateBatch(_ context.Context, input inventory.UpdateBatchRecord) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	record := input
	r.updateRecord = &record
	return nil
}

func (r *fakeBatchRepository) DisableBatch(context.Context, int64) (*inventory.Batch, error) {
	if r.disableErr != nil {
		return nil, r.disableErr
	}
	return r.getBatch, nil
}

func (r *fakeBatchRepository) DeleteBatch(context.Context, int64) error {
	r.deleteCalls++
	return r.deleteErr
}

func (r *fakeBatchRepository) BatchNoExistsForSKU(context.Context, int64, string, *int64) (bool, error) {
	return r.duplicate, nil
}

func (r *fakeBatchRepository) CountBatchReferences(context.Context, int64) (int64, error) {
	return r.refs, nil
}

func (r *fakeBatchRepository) ListBatchWarehouseStocks(context.Context, int64) ([]inventory.BatchWarehouseStock, error) {
	return r.stocks, nil
}

func TestBatchServiceCreateNormalizesInput(t *testing.T) {
	repo := &fakeBatchRepository{
		duplicate: false,
		createID:  11,
		getBatch:  &inventory.Batch{ID: 11, SKUID: 7, BatchNo: "B-1", Status: inventory.BatchStatusActive},
	}
	service := inventory.NewBatchService(repo, newFakeSKUReader(7))

	batch, err := service.CreateBatch(context.Background(), inventory.CreateBatchInput{
		SKUID:   7,
		BatchNo: "  B-1  ",
	})
	if err != nil {
		t.Fatalf("create batch: %v", err)
	}
	if batch.ID != 11 {
		t.Fatalf("expected id 11, got %d", batch.ID)
	}
	if repo.createRecord == nil {
		t.Fatal("expected repository create to be called")
	}
	if repo.createRecord.BatchNo != "B-1" {
		t.Fatalf("expected trimmed batch no, got %q", repo.createRecord.BatchNo)
	}
	if repo.createRecord.Status != inventory.BatchStatusActive {
		t.Fatalf("expected default active status, got %q", repo.createRecord.Status)
	}
	if repo.createRecord.DispatchPriority != 10 {
		t.Fatalf("expected default priority 10, got %d", repo.createRecord.DispatchPriority)
	}
	if repo.createRecord.Remark != nil {
		t.Fatalf("expected nil remark, got %v", repo.createRecord.Remark)
	}
}

func TestBatchServiceCreateRejectsDuplicateBatchNo(t *testing.T) {
	repo := &fakeBatchRepository{duplicate: true}
	service := inventory.NewBatchService(repo, newFakeSKUReader(7))

	_, err := service.CreateBatch(context.Background(), inventory.CreateBatchInput{SKUID: 7, BatchNo: "B-1"})
	if !errors.Is(err, inventory.ErrBatchNoDuplicate) {
		t.Fatalf("expected duplicate error, got %v", err)
	}
	if repo.createRecord != nil {
		t.Fatal("repository create must not be called for duplicates")
	}
}

func TestBatchServiceCreateValidatesDates(t *testing.T) {
	repo := &fakeBatchRepository{createID: 1, getBatch: &inventory.Batch{ID: 1}}
	service := inventory.NewBatchService(repo, newFakeSKUReader(7))

	production := "2026-05-01"
	expiration := "2026-04-01"

	_, err := service.CreateBatch(context.Background(), inventory.CreateBatchInput{
		SKUID:          7,
		BatchNo:        "B-1",
		ProductionDate: &production,
		ExpirationDate: &expiration,
	})
	if !apperr.IsValidationError(err) {
		t.Fatalf("expected validation error for reversed dates, got %v", err)
	}

	invalid := "2026/05/01"
	_, err = service.CreateBatch(context.Background(), inventory.CreateBatchInput{
		SKUID:          7,
		BatchNo:        "B-1",
		ProductionDate: &invalid,
	})
	if !apperr.IsValidationError(err) {
		t.Fatalf("expected validation error for bad date format, got %v", err)
	}
}

func TestBatchServiceCreateRejectsUnknownSKU(t *testing.T) {
	repo := &fakeBatchRepository{createID: 1, getBatch: &inventory.Batch{ID: 1}}
	service := inventory.NewBatchService(repo, newFakeSKUReader())

	_, err := service.CreateBatch(context.Background(), inventory.CreateBatchInput{SKUID: 999, BatchNo: "B-1"})
	if !errors.Is(err, inventory.ErrSKUNotFound) {
		t.Fatalf("expected sku not found, got %v", err)
	}
}

func TestBatchServiceCreateRejectsBlankBatchNo(t *testing.T) {
	repo := &fakeBatchRepository{createID: 1, getBatch: &inventory.Batch{ID: 1}}
	service := inventory.NewBatchService(repo, newFakeSKUReader(7))

	_, err := service.CreateBatch(context.Background(), inventory.CreateBatchInput{SKUID: 7, BatchNo: "   "})
	if !apperr.IsValidationError(err) {
		t.Fatalf("expected validation error for blank batch no, got %v", err)
	}
}

func TestBatchServiceUpdatePreservesStatusAndPriority(t *testing.T) {
	existing := &inventory.Batch{
		ID:               3,
		SKUID:            7,
		BatchNo:          "B-3",
		Status:           inventory.BatchStatusInactive,
		DispatchPriority: 5,
	}
	repo := &fakeBatchRepository{getBatch: existing}
	service := inventory.NewBatchService(repo, newFakeSKUReader(7))

	if _, err := service.UpdateBatch(context.Background(), inventory.UpdateBatchInput{ID: 3}); err != nil {
		t.Fatalf("update batch: %v", err)
	}
	if repo.updateRecord == nil {
		t.Fatal("expected repository update to be called")
	}
	if repo.updateRecord.Status != inventory.BatchStatusInactive {
		t.Fatalf("expected status preserved, got %q", repo.updateRecord.Status)
	}
	if repo.updateRecord.DispatchPriority != 5 {
		t.Fatalf("expected priority preserved, got %d", repo.updateRecord.DispatchPriority)
	}
	if repo.updateRecord.ProductionDate != nil || repo.updateRecord.ExpirationDate != nil {
		t.Fatal("expected dates to be cleared when omitted")
	}
}

func TestBatchServiceUpdateRejectsUnknownBatch(t *testing.T) {
	repo := &fakeBatchRepository{getErr: inventory.ErrNotFound}
	service := inventory.NewBatchService(repo, newFakeSKUReader(7))

	_, err := service.UpdateBatch(context.Background(), inventory.UpdateBatchInput{ID: 99})
	if !errors.Is(err, inventory.ErrBatchNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestBatchServiceDeleteRejectsReferencedBatch(t *testing.T) {
	repo := &fakeBatchRepository{
		getBatch: &inventory.Batch{ID: 3},
		refs:     2,
	}
	service := inventory.NewBatchService(repo, newFakeSKUReader(7))

	err := service.DeleteBatch(context.Background(), 3)
	if !errors.Is(err, inventory.ErrBatchReferenced) {
		t.Fatalf("expected referenced error, got %v", err)
	}
	if repo.deleteCalls != 0 {
		t.Fatal("repository delete must not be called when referenced")
	}
}

func TestBatchServiceDeleteAllowsUnreferencedBatch(t *testing.T) {
	repo := &fakeBatchRepository{getBatch: &inventory.Batch{ID: 3}}
	service := inventory.NewBatchService(repo, newFakeSKUReader(7))

	if err := service.DeleteBatch(context.Background(), 3); err != nil {
		t.Fatalf("delete batch: %v", err)
	}
	if repo.deleteCalls != 1 {
		t.Fatalf("expected one delete call, got %d", repo.deleteCalls)
	}
}

func TestBatchServiceGetReturnsNotFound(t *testing.T) {
	repo := &fakeBatchRepository{getErr: inventory.ErrNotFound}
	service := inventory.NewBatchService(repo, newFakeSKUReader(7))

	_, err := service.GetBatch(context.Background(), 42)
	if !errors.Is(err, inventory.ErrBatchNotFound) {
		t.Fatalf("expected batch not found, got %v", err)
	}
}

func TestBatchServiceListClampsLimitAndValidatesStatus(t *testing.T) {
	repo := &fakeBatchRepository{}
	service := inventory.NewBatchService(repo, newFakeSKUReader(7))

	result, err := service.ListBatches(context.Background(), inventory.ListBatchesFilter{Limit: 1000})
	if err != nil {
		t.Fatalf("list batches: %v", err)
	}
	if result.Limit != inventory.MaxListLimit {
		t.Fatalf("expected limit clamped to %d, got %d", inventory.MaxListLimit, result.Limit)
	}

	invalidStatus := inventory.BatchStatus("archived")
	_, err = service.ListBatches(context.Background(), inventory.ListBatchesFilter{Status: &invalidStatus})
	if !apperr.IsValidationError(err) {
		t.Fatalf("expected validation error for invalid status, got %v", err)
	}
}
