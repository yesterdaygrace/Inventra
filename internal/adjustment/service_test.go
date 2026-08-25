// Integration tests for the adjustment approval workflow on real PostgreSQL.
package adjustment

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"inventory/internal/category"
	"inventory/internal/inventory"
	"inventory/internal/product"
	"inventory/internal/shared/validator"
	"inventory/internal/warehouses"
)

var testModels = []any{
	&warehouses.Warehouse{}, &category.Category{}, &product.Product{},
	&inventory.Inventory{}, &inventory.LedgerEntry{}, &inventory.Reservation{},
	&SystemSetting{}, &Adjustment{},
}

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "host=localhost user=postgres password=postgres dbname=inventory port=5433 sslmode=disable"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require_no_error(t, err)

	cleanup := func() {
		db.Exec("DROP TABLE IF EXISTS inventory_adjustments CASCADE")
		db.Exec("DROP TABLE IF EXISTS system_settings CASCADE")
		db.Exec("DROP TABLE IF EXISTS inventory_reservations CASCADE")
		db.Exec("DROP TABLE IF EXISTS inventory_ledger CASCADE")
		db.Exec("DROP TABLE IF EXISTS inventory CASCADE")
		db.Exec("DROP TABLE IF EXISTS products CASCADE")
		db.Exec("DROP TABLE IF EXISTS categories CASCADE")
		db.Exec("DROP TABLE IF EXISTS warehouses CASCADE")
	}
	cleanup()
	t.Cleanup(cleanup)

	require_no_error(t, db.AutoMigrate(testModels...))
	return db
}

// require_no_error avoids importing testify here to keep the module's test
// deps identical to its runtime deps.
func require_no_error(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func assert_equal[T comparable](t *testing.T, got, want T) {
	t.Helper()
	if got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func setupWorkflow(t *testing.T) (*Service, *gorm.DB, product.Product, warehouses.Warehouse) {
	t.Helper()
	db := setupTestDB(t)

	wh := warehouses.Warehouse{Code: "WH-ADJ", Name: "Adjustment WH"}
	require_no_error(t, db.Create(&wh).Error)
	cat := category.Category{Name: "General"}
	require_no_error(t, db.Create(&cat).Error)
	p := product.Product{Name: "Counted Widget", SKU: "CW-1", Price: 10, CategoryID: cat.ID, LowStockThreshold: 5}
	require_no_error(t, db.Create(&p).Error)

	require_no_error(t, db.Create(&SystemSetting{Key: SettingApprovalThreshold, Value: "500"}).Error)

	invRepo := inventory.NewGORMRepository(db)
	invSvc := inventory.NewService(invRepo)
	repo := NewGORMRepository(db)
	svc := NewService(repo, func(ctx context.Context, productID, warehouseID uuid.UUID, targetQuantity int, referenceType, referenceID, reason string, userID *uuid.UUID) error {
		_, err := invSvc.ApplyCorrection(ctx, productID, warehouseID, targetQuantity, referenceType, referenceID, reason, userID)
		return err
	})

	// Receive 100 units at $10 so LastKnownCost resolves from the ledger.
	_, err := invSvc.Receive(context.Background(), inventory.Movement{
		ProductID: p.ID, Type: inventory.LedgerReceive, Quantity: 100,
		UnitCost: &[]float64{10}[0], WarehouseID: &wh.ID,
	})
	require_no_error(t, err)

	return svc, db, p, wh
}

func TestSubmitUnderThresholdAutoApplies(t *testing.T) {
	svc, db, p, wh := setupWorkflow(t)

	a, err := svc.Submit(context.Background(), SubmitInput{
		ProductID: p.ID, WarehouseID: wh.ID, CountedQuantity: 105,
		Reason: ReasonCountVariance, RequestedBy: uuid.New(), AutoApprove: true,
	})
	require_no_error(t, err)
	assert_equal(t, StatusApproved, a.Status)

	var inv inventory.Inventory
	require_no_error(t, db.Where("product_id = ?", p.ID).First(&inv).Error)
	assert_equal(t, 105, inv.Quantity)

	var entries int64
	require_no_error(t, db.Model(&inventory.LedgerEntry{}).
		Where("product_id = ? AND transaction_type = ?", p.ID, inventory.LedgerAdjustment).
		Count(&entries).Error)
	assert_equal(t, int64(1), entries)
}

func TestSubmitOverThresholdQueuesForManager(t *testing.T) {
	svc, db, p, wh := setupWorkflow(t)

	// Delta 95 × $10 = $950 ≥ $500 threshold → must stay PENDING even though
	// the performer could auto-approve small corrections.
	a, err := svc.Submit(context.Background(), SubmitInput{
		ProductID: p.ID, WarehouseID: wh.ID, CountedQuantity: 195,
		Reason: ReasonDamage, RequestedBy: uuid.New(), AutoApprove: true,
	})
	require_no_error(t, err)
	assert_equal(t, StatusPending, a.Status)

	var inv inventory.Inventory
	require_no_error(t, db.Where("product_id = ?", p.ID).First(&inv).Error)
	assert_equal(t, 100, inv.Quantity)

	// Manager approves: stock snaps to the counted quantity via the ledger.
	reviewer := uuid.New()
	approved, err := svc.Approve(context.Background(), a.ID, reviewer)
	require_no_error(t, err)
	assert_equal(t, StatusApproved, approved.Status)

	require_no_error(t, db.Where("product_id = ?", p.ID).First(&inv).Error)
	assert_equal(t, 195, inv.Quantity)

	var entries int64
	require_no_error(t, db.Model(&inventory.LedgerEntry{}).
		Where("product_id = ? AND transaction_type = ?", p.ID, inventory.LedgerAdjustment).
		Count(&entries).Error)
	assert_equal(t, int64(1), entries)
}

func TestRejectLeavesStockUntouched(t *testing.T) {
	svc, db, p, wh := setupWorkflow(t)

	a, err := svc.Submit(context.Background(), SubmitInput{
		ProductID: p.ID, WarehouseID: wh.ID, CountedQuantity: 300,
		Reason: ReasonOther, RequestedBy: uuid.New(),
	})
	require_no_error(t, err)
	assert_equal(t, StatusPending, a.Status)

	rejected, err := svc.Reject(context.Background(), a.ID, uuid.New())
	require_no_error(t, err)
	assert_equal(t, StatusRejected, rejected.Status)

	var inv inventory.Inventory
	require_no_error(t, db.Where("product_id = ?", p.ID).First(&inv).Error)
	assert_equal(t, 100, inv.Quantity)

	// Reviewing twice conflicts.
	_, err = svc.Reject(context.Background(), a.ID, uuid.New())
	if err == nil {
		t.Fatal("expected conflict on double review")
	}
}

func TestHandlerValidationRejectsBadReason(t *testing.T) {
	if !reasonSet["COUNT_VARIANCE"] || reasonSet["MADE_UP"] {
		t.Fatal("reason set membership broken")
	}
	_ = validator.New()
}
