// Integration tests for the cycle counting workflow on real PostgreSQL.
package cyclecount

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"inventory/internal/adjustment"
	"inventory/internal/category"
	"inventory/internal/inventory"
	"inventory/internal/product"
	"inventory/internal/warehouses"
)

var testModels = []any{
	&warehouses.Warehouse{}, &category.Category{}, &product.Product{},
	&inventory.Inventory{}, &inventory.LedgerEntry{}, &inventory.Reservation{},
	&adjustment.SystemSetting{}, &adjustment.Adjustment{},
	&Plan{}, &Item{},
}

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "host=localhost user=postgres password=postgres dbname=inventory port=5433 sslmode=disable"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	cleanup := func() {
		for _, table := range []string{
			"cycle_count_items", "cycle_count_plans",
			"inventory_adjustments", "system_settings",
			"inventory_reservations", "inventory_ledger", "inventory",
			"products", "categories", "warehouses",
		} {
			db.Exec("DROP TABLE IF EXISTS " + table + " CASCADE")
		}
	}
	cleanup()
	t.Cleanup(cleanup)
	if err := db.AutoMigrate(testModels...); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestCountFlowFilesVarianceAdjustments(t *testing.T) {
	db := setupTestDB(t)

	wh := warehouses.Warehouse{Code: "WH-CC", Name: "Count WH"}
	if err := db.Create(&wh).Error; err != nil {
		t.Fatalf("seed warehouse: %v", err)
	}
	cat := category.Category{Name: "General"}
	if err := db.Create(&cat).Error; err != nil {
		t.Fatalf("seed category: %v", err)
	}
	p1 := product.Product{Name: "Alpha", SKU: "CC-A", Price: 5, CategoryID: cat.ID, LowStockThreshold: 1}
	p2 := product.Product{Name: "Beta", SKU: "CC-B", Price: 7, CategoryID: cat.ID, LowStockThreshold: 1}
	if err := db.Create(&p1).Error; err != nil {
		t.Fatalf("seed p1: %v", err)
	}
	if err := db.Create(&p2).Error; err != nil {
		t.Fatalf("seed p2: %v", err)
	}
	if err := db.Create(&adjustment.SystemSetting{Key: adjustment.SettingApprovalThreshold, Value: "500"}).Error; err != nil {
		t.Fatalf("seed setting: %v", err)
	}

	invRepo := inventory.NewGORMRepository(db)
	invSvc := inventory.NewService(invRepo)
	for _, p := range []product.Product{p1, p2} {
		if _, err := invSvc.Receive(context.Background(), inventory.Movement{
			ProductID: p.ID, Type: inventory.LedgerReceive, Quantity: 50,
			WarehouseID: &wh.ID,
		}); err != nil {
			t.Fatalf("receive %s: %v", p.SKU, err)
		}
	}

	adjRepo := adjustment.NewGORMRepository(db)
	adjSvc := adjustment.NewService(adjRepo, func(ctx context.Context, productID, warehouseID uuid.UUID, targetQuantity int, referenceType, referenceID, reason string, userID *uuid.UUID) error {
		_, err := invSvc.ApplyCorrection(ctx, productID, warehouseID, targetQuantity, referenceType, referenceID, reason, userID)
		return err
	})

	repo := NewGORMRepository(db)
	svc := NewService(repo, adjSvc, adjRepo)
	counter := uuid.New()

	// Plan covering both SKUs.
	plan, err := svc.CreatePlan(context.Background(), CreatePlanInput{
		WarehouseID: wh.ID,
		Name:        "Quarterly shelf audit",
		ProductIDs:  []uuid.UUID{p1.ID, p2.ID},
		CreatedBy:   counter,
	})
	if err != nil {
		t.Fatalf("create plan: %v", err)
	}
	items, err := svc.PlanItems(context.Background(), plan.ID)
	if err != nil {
		t.Fatalf("plan items: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("want 2 items, got %d", len(items))
	}
	for _, it := range items {
		if it.SystemQuantity != 50 {
			t.Fatalf("%s snapshot = %d, want 50", it.ProductSKU, it.SystemQuantity)
		}
		if it.Status != ItemPending {
			t.Fatalf("%s should start PENDING", it.ProductSKU)
		}
	}

	bySku := map[string]*ItemView{}
	for _, it := range items {
		bySku[it.ProductSKU] = it
	}

	// Count Alpha exactly right: no variance, no adjustment.
	if _, err := svc.RecordCount(context.Background(), RecordCountInput{
		ItemID: bySku["CC-A"].ID, CountedQuantity: 50, CountedBy: counter,
	}); err != nil {
		t.Fatalf("count alpha: %v", err)
	}

	// Count Beta short by 3: variance files a PENDING adjustment.
	it, err := svc.RecordCount(context.Background(), RecordCountInput{
		ItemID: bySku["CC-B"].ID, CountedQuantity: 47, CountedBy: counter,
	})
	if err != nil {
		t.Fatalf("count beta: %v", err)
	}
	if it.AdjustmentID == nil {
		t.Fatal("variance must file an adjustment")
	}

	var adj adjustment.Adjustment
	if err := db.First(&adj, "id = ?", *it.AdjustmentID).Error; err != nil {
		t.Fatalf("load adjustment: %v", err)
	}
	if adj.Reason != adjustment.ReasonCountVariance || adj.Status != adjustment.StatusPending {
		t.Fatalf("adjustment = %+v, want PENDING COUNT_VARIANCE", adj)
	}
	if adj.SystemQuantity != 50 || adj.CountedQuantity != 47 {
		t.Fatalf("adjustment quantities = %d/%d, want 50/47", adj.SystemQuantity, adj.CountedQuantity)
	}

	// All items counted → plan completes automatically.
	planAfter, err := repo.GetPlan(context.Background(), plan.ID)
	if err != nil {
		t.Fatalf("reload plan: %v", err)
	}
	if planAfter.Status != PlanCompleted {
		t.Fatalf("plan status = %s, want COMPLETED", planAfter.Status)
	}

	// Stock has NOT moved yet — the variance awaits manager approval.
	var inv inventory.Inventory
	if err := db.Where("product_id = ?", p2.ID).First(&inv).Error; err != nil {
		t.Fatalf("load inventory: %v", err)
	}
	if inv.Quantity != 50 {
		t.Fatalf("stock moved without approval: %d", inv.Quantity)
	}

	// Manager approves the variance; stock snaps to the counted quantity.
	if _, err := adjSvc.Approve(context.Background(), adj.ID, uuid.New()); err != nil {
		t.Fatalf("approve variance: %v", err)
	}
	if err := db.Where("product_id = ?", p2.ID).First(&inv).Error; err != nil {
		t.Fatalf("reload inventory: %v", err)
	}
	if inv.Quantity != 47 {
		t.Fatalf("approved variance not applied: %d", inv.Quantity)
	}
}
