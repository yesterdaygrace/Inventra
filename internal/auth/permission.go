package auth

import "github.com/google/uuid"

// Permission is a named capability (e.g. "product.create") that can be
// granted to roles. The code must be stable because the JWT and the
// Permission middleware both match on it.
type Permission struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	Code        string    `gorm:"type:text;unique;not null"`
	Description string    `gorm:"type:text"`
}

// TableName overrides the default table name.
func (Permission) TableName() string { return "permissions" }

// RolePermission is the join table for the many-to-many between roles
// and permissions.
type RolePermission struct {
	RoleID       uuid.UUID `gorm:"type:uuid;primaryKey;not null"`
	PermissionID uuid.UUID `gorm:"type:uuid;primaryKey;not null"`
}

// TableName overrides the default table name.
func (RolePermission) TableName() string { return "role_permissions" }

// Built-in role names. The four-role model matches the PRD (§41):
// administrators own the platform, warehouse managers run operations,
// staff execute stock movements, viewers get read-only access.
const (
	RoleAdmin            = "ADMIN"
	RoleWarehouseManager = "WAREHOUSE_MANAGER"
	RoleStaff            = "STAFF"
	RoleViewer           = "VIEWER"
)

// Permission codes from the PRD §41 catalog, plus two justified
// extensions: category.* (catalog grouping is first-class here) and
// dashboard.read (dashboard is a distinct read model).
const (
	PermProductRead   = "product.read"
	PermProductCreate = "product.create"
	PermProductUpdate = "product.update"
	PermProductDelete = "product.delete"

	PermCategoryRead   = "category.read"
	PermCategoryCreate = "category.create"
	PermCategoryUpdate = "category.update"
	PermCategoryDelete = "category.delete"

	PermWarehouseRead   = "warehouse.read"
	PermWarehouseManage = "warehouse.manage"

	PermInventoryRead     = "inventory.read"
	PermInventoryReceive  = "inventory.receive"
	PermInventoryIssue    = "inventory.issue"
	PermInventoryAdjust   = "inventory.adjust"
	PermInventoryTransfer = "inventory.transfer"

	PermUserManage = "user.manage"
	PermAuditRead  = "audit.read"

	PermReportRead   = "report.read"
	PermReportExport = "report.export"

	PermDashboardRead = "dashboard.read"
)

// PermissionSetForRole returns the permission codes granted to a built-in
// role. It is shared by the seed CLI, the test doubles, and any
// role-impersonation path, so the catalog stays in one place.
//
// Mapping follows PRD §4 responsibilities:
//   - ADMIN: full control including users and audit.
//   - WAREHOUSE_MANAGER: runs operations — catalog writes except delete,
//     warehouse management, every inventory operation incl. adjust,
//     reports with export, audit visibility.
//   - STAFF: executes stock movements (receive/issue/transfer) and may
//     submit adjustments; approval gating arrives with the adjustment
//     workflow and is enforced by role, not by this code.
//   - VIEWER: read-only across operational data and reports.
func PermissionSetForRole(role string) []string {
	switch role {
	case RoleAdmin:
		return []string{
			PermProductRead, PermProductCreate, PermProductUpdate, PermProductDelete,
			PermCategoryRead, PermCategoryCreate, PermCategoryUpdate, PermCategoryDelete,
			PermWarehouseRead, PermWarehouseManage,
			PermInventoryRead, PermInventoryReceive, PermInventoryIssue, PermInventoryAdjust, PermInventoryTransfer,
			PermUserManage, PermAuditRead,
			PermReportRead, PermReportExport,
			PermDashboardRead,
		}
	case RoleWarehouseManager:
		return []string{
			PermProductRead, PermProductCreate, PermProductUpdate,
			PermCategoryRead,
			PermWarehouseRead, PermWarehouseManage,
			PermInventoryRead, PermInventoryReceive, PermInventoryIssue, PermInventoryAdjust, PermInventoryTransfer,
			PermAuditRead,
			PermReportRead, PermReportExport,
			PermDashboardRead,
		}
	case RoleStaff:
		return []string{
			PermProductRead,
			PermCategoryRead,
			PermWarehouseRead,
			PermInventoryRead, PermInventoryReceive, PermInventoryIssue, PermInventoryAdjust, PermInventoryTransfer,
			PermReportRead,
			PermDashboardRead,
		}
	case RoleViewer:
		return []string{
			PermProductRead,
			PermCategoryRead,
			PermWarehouseRead,
			PermInventoryRead,
			PermReportRead,
			PermDashboardRead,
		}
	default:
		return nil
	}
}
