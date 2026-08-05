// GORM-backed implementation of the inventory Repository interface.
package inventory

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	sharederr "inventory/internal/shared/errors"
)

// GORMRepository persists inventory using GORM.
type GORMRepository struct {
	db *gorm.DB
}

// NewGORMRepository constructs a repository over the given DB handle.
func NewGORMRepository(db *gorm.DB) *GORMRepository {
	return &GORMRepository{db: db}
}

// StockIn records an IN movement and increments the product quantity in a
// single DB transaction. The inventory row is upserted and projects with no
// existing row start from zero, so the first IN creates it.
func (r *GORMRepository) StockIn(m Movement) (*Inventory, error) {
	var result Inventory
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var inv Inventory
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("product_id = ?", m.ProductID).First(&inv).Error
		switch {
		case err == nil:
			inv.Quantity += m.Quantity
		case err == gorm.ErrRecordNotFound:
			inv = Inventory{ProductID: m.ProductID, Quantity: m.Quantity}
		default:
			return err
		}

		if err := tx.Save(&inv).Error; err != nil {
			return err
		}
		if err := tx.Create(&InventoryTransaction{
			ProductID: m.ProductID,
			Type:      "IN",
			Quantity:  m.Quantity,
			UnitCost:  m.UnitCost,
			Note:      m.Note,
			UserID:    m.UserID,
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

// StockOut records an OUT movement and decrements the product quantity in a
// single transaction. It rejects any draw that would push stock below zero,
// returning ErrConflict and rolling back so no partial history row remains.
func (r *GORMRepository) StockOut(m Movement) (*Inventory, error) {
	var result Inventory
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var inv Inventory
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("product_id = ?", m.ProductID).First(&inv).Error
		if err == gorm.ErrRecordNotFound {
			return sharederr.ErrConflict
		}
		if err != nil {
			return err
		}

		if inv.Quantity < m.Quantity {
			return sharederr.ErrConflict
		}
		inv.Quantity -= m.Quantity

		if err := tx.Save(&inv).Error; err != nil {
			return err
		}
		if err := tx.Create(&InventoryTransaction{
			ProductID: m.ProductID,
			Type:      "OUT",
			Quantity:  m.Quantity,
			UnitCost:  m.UnitCost,
			Note:      m.Note,
			UserID:    m.UserID,
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
