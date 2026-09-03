// GORM-backed implementation of the inventory Repository interface.
package inventory

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"inventory/internal/shared/dbutil"
	sharederr "inventory/internal/shared/errors"
)

// defaultWarehouseCode is the code of the seeded fallback warehouse used when
// a stock movement does not specify a warehouse.
const defaultWarehouseCode = "DEFAULT"

// GORMRepository persists inventory using GORM.
type GORMRepository struct {
	db *gorm.DB
}

// NewGORMRepository constructs a repository over the given DB handle.
func NewGORMRepository(db *gorm.DB) *GORMRepository {
	return &GORMRepository{db: db}
}

// DefaultWarehouse returns the ID of the seeded DEFAULT warehouse. It errors
// with ErrNotFound when the seed has not run yet.
func (r *GORMRepository) DefaultWarehouse(ctx context.Context) (uuid.UUID, error) {
	var row struct {
		ID uuid.UUID
	}
	err := r.db.WithContext(ctx).Table("warehouses").
		Select("id").Where("code = ?", defaultWarehouseCode).Take(&row).Error
	if err == gorm.ErrRecordNotFound {
		return uuid.Nil, sharederr.ErrNotFound
	}
	if err != nil {
		return uuid.Nil, err
	}
	return row.ID, nil
}

// resolveWarehouse maps a movement's optional warehouse to a concrete ID,
// falling back to the seeded DEFAULT warehouse when omitted.
func (r *GORMRepository) resolveWarehouse(ctx context.Context, wh *uuid.UUID) (uuid.UUID, error) {
	if wh != nil {
		return *wh, nil
	}
	return r.DefaultWarehouse(ctx)
}

// totalCostOf multiplies quantity by unit cost, returning nil when no unit
// cost is recorded (e.g. issues before the costing engine assigns one).
// TotalCostOf multiplies quantity by unit cost, returning nil when no unit
// cost is recorded. Exported for the seed CLI and future costing work.
func TotalCostOf(qty int, unit *float64) *float64 {
	if unit == nil {
		return nil
	}
	v := float64(qty) * *unit
	return &v
}

// expireStaleReservations flips ACTIVE reservations past their expiry to
// EXPIRED and releases their reserved quantity from the inventory row.
// Caller must hold the inventory row lock and run this inside its
// transaction; the released total is derived from exactly the rows flipped,
// so repeated calls never double-subtract. Returns the released quantity.
func expireStaleReservations(tx *gorm.DB, productID, warehouseID uuid.UUID) (int, error) {
	var flipped []int
	if err := tx.Raw(`
		UPDATE inventory_reservations SET status = ?, updated_at = now()
		WHERE product_id = ? AND warehouse_id = ? AND status = ?
		  AND expires_at IS NOT NULL AND expires_at < now()
		RETURNING quantity`,
		ReservationExpired, productID, warehouseID, ReservationActive).
		Scan(&flipped).Error; err != nil {
		return 0, err
	}
	released := 0
	for _, q := range flipped {
		released += q
	}
	if released == 0 {
		return 0, nil
	}
	if err := tx.Exec(`
		UPDATE inventory SET reserved_quantity = GREATEST(0, reserved_quantity - ?),
			version = version + 1
		WHERE product_id = ? AND warehouse_id = ?`,
		released, productID, warehouseID).Error; err != nil {
		return 0, err
	}
	return released, nil
}

// activeReservedQuantity sums ACTIVE reservation quantities for a pair.
func activeReservedQuantity(tx *gorm.DB, productID, warehouseID uuid.UUID) (int, error) {
	var sum int
	err := tx.Model(&Reservation{}).
		Select("COALESCE(SUM(quantity), 0)").
		Where("product_id = ? AND warehouse_id = ? AND status = ?", productID, warehouseID, ReservationActive).
		Scan(&sum).Error
	return sum, err
}

// Receive records a RECEIVE ledger entry and increments the product quantity
// in a single DB transaction, scoped to the movement's warehouse (DEFAULT
// when omitted). The inventory row is upserted per (product, warehouse) pair
// and products with no existing row start from zero, so the first receive
// creates it.
func (r *GORMRepository) Receive(ctx context.Context, m Movement) (*Inventory, error) {
	whID, err := r.resolveWarehouse(ctx, m.WarehouseID)
	if err != nil {
		return nil, err
	}

	var result Inventory
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var inv Inventory
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("product_id = ? AND warehouse_id = ?", m.ProductID, whID).First(&inv).Error
		switch err {
		case nil:
			inv.Quantity += m.Quantity
			inv.Version++
		case gorm.ErrRecordNotFound:
			inv = Inventory{ProductID: m.ProductID, WarehouseID: whID, Quantity: m.Quantity, Version: 1}
		default:
			return err
		}

		if err := tx.Save(&inv).Error; err != nil {
			return err
		}
		if err := tx.Create(&LedgerEntry{
			ProductID:       m.ProductID,
			WarehouseID:     whID,
			TransactionType: LedgerReceive,
			Direction:       "IN",
			Quantity:        m.Quantity,
			UnitCost:        m.UnitCost,
			TotalCost:       TotalCostOf(m.Quantity, m.UnitCost),
			Note:            m.Note,
			PerformedBy:     m.UserID,
			ReferenceType:   m.ReferenceType,
			ReferenceID:     m.ReferenceID,
			Reason:          m.Reason,
		}).Error; err != nil {
			return err
		}
		result = inv
		return nil
	})
	if err != nil {
		if dbutil.IsForeignKeyViolation(err) {
			return nil, sharederr.ErrNotFound
		}
		return nil, err
	}
	return &result, nil
}

// Issue records an ISSUE ledger entry and decrements the product quantity in
// a single transaction, scoped to the movement's warehouse (DEFAULT when
// omitted). It rejects any draw that would push stock below zero, returning
// ErrInsufficientStock and rolling back so no partial ledger row remains.
func (r *GORMRepository) Issue(ctx context.Context, m Movement) (*Inventory, error) {
	whID, err := r.resolveWarehouse(ctx, m.WarehouseID)
	if err != nil {
		return nil, err
	}

	var result Inventory
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var inv Inventory
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("product_id = ? AND warehouse_id = ?", m.ProductID, whID).First(&inv).Error
		if err == gorm.ErrRecordNotFound {
			return sharederr.ErrConflict
		}
		if err != nil {
			return err
		}

		// Lazy expiry first, then availability = on hand − active reserved
		// (PRD §20: issued quantity must come from available stock).
		released, err := expireStaleReservations(tx, m.ProductID, whID)
		if err != nil {
			return err
		}
		if released > 0 {
			if err := tx.First(&inv, "product_id = ? AND warehouse_id = ?", m.ProductID, whID).Error; err != nil {
				return err
			}
		}
		active, err := activeReservedQuantity(tx, m.ProductID, whID)
		if err != nil {
			return err
		}
		if inv.Quantity-active < m.Quantity {
			return sharederr.ErrInsufficientStock
		}
		inv.Quantity -= m.Quantity
		inv.Version++

		if err := tx.Save(&inv).Error; err != nil {
			return err
		}
		if err := tx.Create(&LedgerEntry{
			ProductID:       m.ProductID,
			WarehouseID:     whID,
			TransactionType: LedgerIssue,
			Direction:       "OUT",
			Quantity:        m.Quantity,
			UnitCost:        m.UnitCost,
			TotalCost:       TotalCostOf(m.Quantity, m.UnitCost),
			Note:            m.Note,
			PerformedBy:     m.UserID,
			ReferenceType:   m.ReferenceType,
			ReferenceID:     m.ReferenceID,
			Reason:          m.Reason,
		}).Error; err != nil {
			return err
		}
		result = inv
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// Transfer moves stock between two warehouses in a single transaction: it
// locks and decrements the source row, upserts the destination row, and
// writes two history rows (OUT from source, IN to destination) sharing one
// transfer_id. 404 when the product or either warehouse does not exist; 409
// when the source lacks the requested quantity.
func (r *GORMRepository) Transfer(ctx context.Context, t Transfer) (*Inventory, error) {
	transferID := uuid.New()
	var result Inventory

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Existence checks → 404 semantics.
		var prodCount int64
		if err := tx.Table("products").Where("id = ?", t.ProductID).Count(&prodCount).Error; err != nil {
			return err
		}
		if prodCount == 0 {
			return sharederr.ErrNotFound
		}
		for _, wh := range []uuid.UUID{t.FromWarehouseID, t.ToWarehouseID} {
			var whCount int64
			if err := tx.Table("warehouses").Where("id = ?", wh).Count(&whCount).Error; err != nil {
				return err
			}
			if whCount == 0 {
				return sharederr.ErrNotFound
			}
		}

		// Lock the source row, lazily expire reservations, then check
		// availability = on hand − active reserved at the source.
		var src Inventory
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("product_id = ? AND warehouse_id = ?", t.ProductID, t.FromWarehouseID).First(&src).Error
		if err == gorm.ErrRecordNotFound {
			return sharederr.ErrConflict
		}
		if err != nil {
			return err
		}
		released, err := expireStaleReservations(tx, t.ProductID, t.FromWarehouseID)
		if err != nil {
			return err
		}
		if released > 0 {
			if err := tx.First(&src, "product_id = ? AND warehouse_id = ?", t.ProductID, t.FromWarehouseID).Error; err != nil {
				return err
			}
		}
		active, err := activeReservedQuantity(tx, t.ProductID, t.FromWarehouseID)
		if err != nil {
			return err
		}
		if src.Quantity-active < t.Quantity {
			return sharederr.ErrInsufficientStock
		}
		src.Quantity -= t.Quantity
		src.Version++
		if err := tx.Save(&src).Error; err != nil {
			return err
		}

		// Upsert the destination row (lock, then increment or create).
		var dst Inventory
		err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("product_id = ? AND warehouse_id = ?", t.ProductID, t.ToWarehouseID).First(&dst).Error
		switch err {
		case nil:
			dst.Quantity += t.Quantity
			dst.Version++
			if err := tx.Save(&dst).Error; err != nil {
				return err
			}
		case gorm.ErrRecordNotFound:
			dst = Inventory{ProductID: t.ProductID, WarehouseID: t.ToWarehouseID, Quantity: t.Quantity, Version: 1}
			if err := tx.Create(&dst).Error; err != nil {
				return err
			}
		default:
			return err
		}

		// Two ledger rows sharing one transfer_id.
		if err := tx.Create(&LedgerEntry{
			ProductID:       t.ProductID,
			WarehouseID:     t.FromWarehouseID,
			TransactionType: LedgerTransferOut,
			Direction:       "OUT",
			Quantity:        t.Quantity,
			Note:            t.Note,
			PerformedBy:     t.UserID,
			TransferID:      &transferID,
			ReferenceType:   t.ReferenceType,
			ReferenceID:     t.ReferenceID,
			Reason:          t.Reason,
		}).Error; err != nil {
			return err
		}
		if err := tx.Create(&LedgerEntry{
			ProductID:       t.ProductID,
			WarehouseID:     t.ToWarehouseID,
			TransactionType: LedgerTransferIn,
			Direction:       "IN",
			Quantity:        t.Quantity,
			Note:            t.Note,
			PerformedBy:     t.UserID,
			TransferID:      &transferID,
			ReferenceType:   t.ReferenceType,
			ReferenceID:     t.ReferenceID,
			Reason:          t.Reason,
		}).Error; err != nil {
			return err
		}

		result = dst
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// List returns a filtered, sorted, paginated joined inventory view (every
// product left-joined with its stock rows) plus the total match count. All
// dynamic values are parameterized, so input cannot be injected. Without a
// warehouse filter quantities are aggregated across all warehouses; with one,
// the view is scoped to that warehouse's rows.
func (r *GORMRepository) List(ctx context.Context, q ListQuery) ([]*InventoryView, int64, error) {
	join := "LEFT JOIN inventory ON inventory.product_id = products.id"
	joinArgs := []any{}
	if q.WarehouseID != nil {
		join += " AND inventory.warehouse_id = ?"
		joinArgs = append(joinArgs, *q.WarehouseID)
	}

	base := r.db.WithContext(ctx).Table("products").Joins(join, joinArgs...)

	if q.ProductID != uuid.Nil {
		base = base.Where("products.id = ?", q.ProductID)
	}
	if q.Search != "" {
		like := "%" + strings.ToLower(q.Search) + "%"
		base = base.Where("(LOWER(products.name) LIKE ? OR LOWER(products.sku) LIKE ?)", like, like)
	}

	// Total: count distinct product rows matching filters (products may join
	// to multiple inventory rows per warehouse).
	grouped := base.Select("products.id").Group("products.id")
	if q.LowStock {
		grouped = grouped.Having("COALESCE(SUM(inventory.quantity), 0) <= products.low_stock_threshold")
	}
	var total int64
	if err := r.db.WithContext(ctx).Table("(?) AS filtered", grouped).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Page data: aggregate quantity per product.
	db := base.Select(strings.Join([]string{
		"products.id AS product_id",
		"products.sku AS product_sku",
		"products.name AS product_name",
		"COALESCE(SUM(inventory.quantity), 0) AS quantity",
		"COALESCE(SUM(inventory.reserved_quantity), 0) AS reserved_quantity",
		"COALESCE(MAX(inventory.version), 0) AS version",
		"COALESCE(MAX(inventory.updated_at), products.created_at) AS updated_at",
	}, ", ")).
		Group("products.id, products.sku, products.name, products.created_at")
	if q.LowStock {
		db = db.Having("COALESCE(SUM(inventory.quantity), 0) <= products.low_stock_threshold")
	}

	p, per := dbutil.NormalizePage(q.Page, q.PerPage)
	var views []*InventoryView
	if err := db.Order("products.name ASC").
		Offset((p - 1) * per).
		Limit(per).
		Scan(&views).Error; err != nil {
		return nil, 0, err
	}
	return views, total, nil
}

// Ledger returns a filtered, paginated ledger history joined with product
// identity. The running balance per (product, warehouse) pair is computed on
// read with a window function: inflows (everything except ISSUE and
// TRANSFER_OUT) add, outflows subtract — so the balance can never disagree
// with the rows it summarizes.
func (r *GORMRepository) Ledger(ctx context.Context, q LedgerQuery) ([]*LedgerView, int64, error) {
	if q.Type != "" && !ledgerTypeSet[q.Type] {
		return nil, 0, sharederr.ErrValidation
	}

	balanceExpr := `SUM(CASE WHEN l.direction = 'OUT' THEN -l.quantity ELSE l.quantity END)
		OVER (PARTITION BY l.product_id, l.warehouse_id ORDER BY l.created_at, l.id)`

	db := r.db.WithContext(ctx).Table("inventory_ledger AS l").
		Select(strings.Join([]string{
			"l.id", "l.product_id", "p.sku AS product_sku",
			"p.name AS product_name", "l.transaction_type", "l.direction", "l.quantity",
			"(" + balanceExpr + ") AS balance",
			"l.unit_cost", "l.total_cost", "l.note", "l.performed_by",
			"l.warehouse_id", "l.transfer_id",
			"l.reference_type", "l.reference_id", "l.reason", "l.created_at",
		}, ", ")).
		Joins("JOIN products p ON p.id = l.product_id")

	if q.ProductID != uuid.Nil {
		db = db.Where("l.product_id = ?", q.ProductID)
	}
	if q.WarehouseID != nil {
		db = db.Where("l.warehouse_id = ?", *q.WarehouseID)
	}
	if q.Type != "" {
		db = db.Where("l.transaction_type = ?", q.Type)
	}

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	p, per := dbutil.NormalizePage(q.Page, q.PerPage)
	var views []*LedgerView
	if err := db.Order("l.created_at DESC, l.id DESC").
		Offset((p - 1) * per).
		Limit(per).
		Scan(&views).Error; err != nil {
		return nil, 0, err
	}
	return views, total, nil
}

// CreateReservation reserves available stock for a reference, in a single
// transaction: lock the inventory row, lazily expire stale reservations,
// verify availability, bump reserved_quantity, and insert the ACTIVE row.
func (r *GORMRepository) CreateReservation(ctx context.Context, rsv Reservation) (*Reservation, error) {
	var result Reservation
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Existence checks → 404 semantics.
		var prodCount, whCount int64
		if err := tx.Table("products").Where("id = ?", rsv.ProductID).Count(&prodCount).Error; err != nil {
			return err
		}
		if prodCount == 0 {
			return sharederr.ErrNotFound
		}
		if err := tx.Table("warehouses").Where("id = ?", rsv.WarehouseID).Count(&whCount).Error; err != nil {
			return err
		}
		if whCount == 0 {
			return sharederr.ErrNotFound
		}

		var inv Inventory
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("product_id = ? AND warehouse_id = ?", rsv.ProductID, rsv.WarehouseID).First(&inv).Error
		if err == gorm.ErrRecordNotFound {
			// No stock row means nothing on hand and nothing reserved.
			return sharederr.ErrInsufficientStock
		}
		if err != nil {
			return err
		}

		if _, err := expireStaleReservations(tx, rsv.ProductID, rsv.WarehouseID); err != nil {
			return err
		}
		active, err := activeReservedQuantity(tx, rsv.ProductID, rsv.WarehouseID)
		if err != nil {
			return err
		}
		if inv.Quantity-active < rsv.Quantity {
			return sharederr.ErrInsufficientStock
		}

		inv.ReservedQuantity += rsv.Quantity
		inv.Version++
		if err := tx.Save(&inv).Error; err != nil {
			return err
		}
		if err := tx.Create(&rsv).Error; err != nil {
			return err
		}
		result = rsv
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// ReleaseReservation returns a reservation's quantity to available stock.
func (r *GORMRepository) ReleaseReservation(ctx context.Context, id uuid.UUID) (*Reservation, error) {
	var result Reservation
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var rsv Reservation
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&rsv, "id = ?", id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return sharederr.ErrNotFound
			}
			return err
		}
		if rsv.Status != ReservationActive {
			return sharederr.ErrConflict
		}

		var inv Inventory
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("product_id = ? AND warehouse_id = ?", rsv.ProductID, rsv.WarehouseID).First(&inv).Error; err != nil {
			return err
		}
		inv.ReservedQuantity = max(0, inv.ReservedQuantity-rsv.Quantity)
		inv.Version++
		if err := tx.Save(&inv).Error; err != nil {
			return err
		}

		rsv.Status = ReservationReleased
		if err := tx.Save(&rsv).Error; err != nil {
			return err
		}
		result = rsv
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// ConsumeReservation converts an ACTIVE reservation into an ISSUE: stock and
// reserved_quantity both drop by the reserved amount, one ISSUE ledger entry
// is written referencing the reservation, and the reservation is CONSUMED —
// all atomically.
func (r *GORMRepository) ConsumeReservation(ctx context.Context, id uuid.UUID, userID *uuid.UUID) (*Reservation, *Inventory, error) {
	var rsvResult Reservation
	var invResult Inventory
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var rsv Reservation
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&rsv, "id = ?", id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return sharederr.ErrNotFound
			}
			return err
		}
		if rsv.Status != ReservationActive {
			return sharederr.ErrConflict
		}

		var inv Inventory
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("product_id = ? AND warehouse_id = ?", rsv.ProductID, rsv.WarehouseID).First(&inv).Error; err != nil {
			return err
		}
		if inv.Quantity < rsv.Quantity {
			return sharederr.ErrInsufficientStock
		}
		inv.Quantity -= rsv.Quantity
		inv.ReservedQuantity = max(0, inv.ReservedQuantity-rsv.Quantity)
		inv.Version++
		if err := tx.Save(&inv).Error; err != nil {
			return err
		}

		refType := "reservation"
		refID := rsv.ID.String()
		if err := tx.Create(&LedgerEntry{
			ProductID:       rsv.ProductID,
			WarehouseID:     rsv.WarehouseID,
			TransactionType: LedgerIssue,
			Direction:       "OUT",
			Quantity:        rsv.Quantity,
			Note:            nil,
			PerformedBy:     userID,
			ReferenceType:   &refType,
			ReferenceID:     &refID,
		}).Error; err != nil {
			return err
		}

		rsv.Status = ReservationConsumed
		if err := tx.Save(&rsv).Error; err != nil {
			return err
		}
		rsvResult = rsv
		invResult = inv
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return &rsvResult, &invResult, nil
}

// ReservationQuery filters and paginates reservations.
type ReservationQuery struct {
	ProductID   uuid.UUID
	WarehouseID *uuid.UUID
	Status      string
	Page        int
	PerPage     int
}

// ReservationView is a reservation joined with its product identity.
type ReservationView struct {
	ID            uuid.UUID `gorm:"column:id"`
	ProductID     uuid.UUID `gorm:"column:product_id"`
	ProductSKU    string    `gorm:"column:product_sku"`
	ProductName   string    `gorm:"column:product_name"`
	WarehouseID   uuid.UUID `gorm:"column:warehouse_id"`
	Quantity      int       `gorm:"column:quantity"`
	ReferenceType string    `gorm:"column:reference_type"`
	ReferenceID   string    `gorm:"column:reference_id"`
	Status        string    `gorm:"column:status"`
	ExpiresAt     *string   `gorm:"column:expires_at"`
	CreatedAt     string    `gorm:"column:created_at"`
}

// Reservations lists reservations with lazy expiry applied to the filtered
// scope first, so expired rows surface as EXPIRED without a worker.
func (r *GORMRepository) Reservations(ctx context.Context, q ReservationQuery) ([]*ReservationView, int64, error) {
	if q.Status != "" && !reservationStatusSet[q.Status] {
		return nil, 0, sharederr.ErrValidation
	}

	db := r.db.WithContext(ctx).Table("inventory_reservations AS r").
		Select(strings.Join([]string{
			"r.id", "r.product_id", "p.sku AS product_sku", "p.name AS product_name",
			"r.warehouse_id", "r.quantity", "r.reference_type", "r.reference_id",
			"r.status", "to_char(r.expires_at, 'YYYY-MM-DD\"T\"HH24:MI:SS\"Z\"') AS expires_at",
			"to_char(r.created_at, 'YYYY-MM-DD\"T\"HH24:MI:SS\"Z\"') AS created_at",
		}, ", ")).
		Joins("JOIN products p ON p.id = r.product_id")

	if q.ProductID != uuid.Nil {
		db = db.Where("r.product_id = ?", q.ProductID)
	}
	if q.WarehouseID != nil {
		db = db.Where("r.warehouse_id = ?", *q.WarehouseID)
	}
	if q.Status != "" {
		db = db.Where("r.status = ?", q.Status)
	} else {
		db = db.Where("r.status = ?", ReservationActive)
	}

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	p, per := dbutil.NormalizePage(q.Page, q.PerPage)
	var views []*ReservationView
	if err := db.Order("r.created_at DESC, r.id DESC").
		Offset((p - 1) * per).
		Limit(per).
		Scan(&views).Error; err != nil {
		return nil, 0, err
	}
	return views, total, nil
}

// ApplyCorrection sets stock to an exact counted quantity and writes a
// single ADJUSTMENT ledger entry capturing the delta — atomically. This is
// the only path by which stock may deviate from movements (PRD §23).
func (r *GORMRepository) ApplyCorrection(ctx context.Context, productID, warehouseID uuid.UUID, targetQuantity int, referenceType, referenceID, reason string, userID *uuid.UUID) (*Inventory, error) {
	var result Inventory
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var inv Inventory
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("product_id = ? AND warehouse_id = ?", productID, warehouseID).First(&inv).Error
		switch err {
		case nil:
			// existing row: adjust to target below
		case gorm.ErrRecordNotFound:
			// Correcting a SKU with no stock row creates it at the target.
			inv = Inventory{ProductID: productID, WarehouseID: warehouseID, Quantity: 0, Version: 1}
		default:
			return err
		}

		before := inv.Quantity
		delta := targetQuantity - before

		inv.Quantity = targetQuantity
		inv.Version++
		if err := tx.Save(&inv).Error; err != nil {
			return err
		}

		qty := delta
		if qty < 0 {
			qty = -qty
		}
		if qty == 0 {
			// Target equals current stock: nothing to record.
			result = inv
			return nil
		}
		refType := referenceType
		refID := referenceID
		reasonText := reason
		if err := tx.Create(&LedgerEntry{
			ProductID:       productID,
			WarehouseID:     warehouseID,
			TransactionType: LedgerAdjustment,
			Direction:       "IN",
			Quantity:        qty,
			PerformedBy:     userID,
			ReferenceType:   &refType,
			ReferenceID:     &refID,
			Reason:          &reasonText,
		}).Error; err != nil {
			return err
		}
		result = inv
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}
