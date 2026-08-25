# Inventra --- PRD, UI/UX, System Design & Engineering Specification

**Version:** 3.1\
**Status:** Final Design Baseline\
**Product:** Inventra\
**Domain:** Inventory Control & Inventory Intelligence\
**Architecture:** Modular Monolith\
**Primary Backend:** Go\
**Primary Database:** PostgreSQL 17

------------------------------------------------------------------------

# 1. Product Definition

Inventra is a multi-warehouse inventory control and intelligence
platform designed to maintain accurate, traceable, auditable, and
actionable inventory information.

Inventra answers five core questions:

1.  **What inventory do we have?**
2.  **Where is it?**
3.  **What changed and why?**
4.  **What is the inventory worth?**
5.  **When should we replenish it?**

Inventra is deliberately different from a logistics platform.

### Inventra

> Inventory control, stock accuracy, valuation, counting, traceability,
> and replenishment intelligence.

### Vayra

> Logistics, fulfillment, shipment orchestration, drivers, routing,
> delivery, and real-time logistics.

Inventra must remain the inventory-control sibling, not Vayra Lite.

------------------------------------------------------------------------

# 2. Product Goals

## Primary Goals

-   Maintain accurate stock quantities across warehouses.
-   Make every stock movement traceable.
-   Prevent inventory corruption during concurrent operations.
-   Support reservations without becoming an order-fulfillment system.
-   Support batch, lot, and expiry tracking.
-   Support FIFO and weighted-average inventory costing.
-   Support cycle counting and controlled adjustments.
-   Provide inventory valuation and aging analysis.
-   Detect dead stock and low-stock conditions.
-   Generate replenishment recommendations.
-   Provide ABC inventory analysis.
-   Provide practical demand-planning capabilities.
-   Provide strong RBAC and auditability.
-   Demonstrate production-quality Go engineering.

## Non-Goals

Do not build these as core Inventra capabilities:

-   GPS tracking
-   Driver management
-   Route optimization
-   Delivery tracking
-   Shipment orchestration
-   Full delivery management
-   Kafka
-   RabbitMQ
-   NATS as the core architecture
-   Microservices
-   gRPC
-   Elasticsearch
-   Kubernetes
-   Service mesh
-   AI-first forecasting

Infrastructure must only be introduced when a measurable requirement
justifies it.

------------------------------------------------------------------------

# 3. Product Identity

Inventra's product identity is:

``` text
CONTROL
  ↓
TRACE
  ↓
VALUE
  ↓
COUNT
  ↓
ANALYZE
  ↓
REPLENISH
```

The product should feel like:

-   modern enterprise SaaS
-   warehouse operations software
-   inventory intelligence
-   precise and trustworthy
-   data-dense but not cluttered

It should not feel like:

-   a generic admin template
-   a delivery dashboard
-   a social application
-   a dashboard filled with meaningless KPI cards

------------------------------------------------------------------------

# 4. User Roles

## Administrator

Responsibilities:

-   Users
-   Roles
-   Permissions
-   System configuration
-   Audit review

## Warehouse Manager

Responsibilities:

-   Warehouse inventory
-   Stock movements
-   Transfers
-   Cycle counts
-   Adjustment approvals
-   Replenishment
-   Inventory analytics

## Inventory Staff

Responsibilities:

-   Receive stock
-   Issue stock
-   Count inventory
-   Submit adjustments
-   View inventory

## Management / Viewer

Responsibilities:

-   Dashboard
-   Inventory valuation
-   Analytics
-   Reports
-   Stock health

------------------------------------------------------------------------

# 5. Technology Stack

## Backend

-   Go 1.24
-   Gin
-   GORM
-   PostgreSQL 17
-   JWT HS256
-   bcrypt
-   Zap structured logging

## Frontend

-   React 18
-   TypeScript
-   Vite 6.4
-   TanStack React Query
-   React Hook Form
-   Zod
-   Tailwind CSS
-   shadcn/ui
-   Lucide

## DevOps

-   Docker
-   Docker Compose
-   GitHub Actions
-   Swagger / Swaggo
-   golangci-lint v2

## Database

-   PostgreSQL 17
-   Versioned database migrations
-   Recommended migration tool: `golang-migrate`

PostgreSQL is the source of truth.

GORM is the ORM/data-access layer.

Production schema evolution must not depend on GORM AutoMigrate.

------------------------------------------------------------------------

# 6. Architecture Decision

Inventra remains a **modular monolith**.

``` text
React + TypeScript
        │
        ▼
TanStack Query
        │
        ▼
      REST
        │
        ▼
   Gin HTTP API
        │
        ▼
┌──────────────────────────┐
│      Modular Monolith    │
│                          │
│ Auth                     │
│ User                     │
│ Product                  │
│ Category                 │
│ Warehouse                │
│ Inventory                │
│ Dashboard                │
│ Report                   │
│ Activity / Audit         │
└────────────┬─────────────┘
             │
           GORM
             │
             ▼
      PostgreSQL 17
```

Cross-cutting capabilities:

``` text
Authentication
Authorization
Validation
Transactions
Idempotency
Request IDs
Structured Logging
Audit Logging
Pagination
Error Handling
```

------------------------------------------------------------------------

# 7. Module Structure

Recommended backend structure:

``` text
internal/
├── auth/
│   ├── handler/
│   ├── service/
│   ├── repository/
│   ├── model/
│   └── dto/
│
├── user/
├── product/
├── category/
├── warehouse/
│
├── inventory/
│   ├── handler/
│   ├── service/
│   ├── repository/
│   ├── model/
│   ├── dto/
│   ├── validator/
│   └── errors/
│
├── dashboard/
├── report/
├── activitylog/
│
└── shared/
    ├── database/
    ├── middleware/
    ├── auth/
    ├── logger/
    ├── response/
    ├── pagination/
    ├── errors/
    └── idempotency/
```

Not every module needs every folder.

Do not create abstractions only to make the architecture look larger.

------------------------------------------------------------------------

# 8. Module Boundary Rules

Modules must not directly access another module's internal repository or
database implementation.

### Forbidden

``` text
Inventory
   ↓
Product Repository
   ↓
GORM DB
```

### Preferred

``` text
Inventory Service
   ↓
Product Service / Contract
   ↓
Product Module
```

Rules:

-   Domain modules own their business logic.
-   Repositories are internal to their module.
-   Cross-module communication uses explicit interfaces/services.
-   Shared contains only genuinely cross-cutting functionality.
-   Avoid circular dependencies.
-   Avoid direct cross-module database manipulation.

------------------------------------------------------------------------

# 9. Core Domain Model

``` text
Product
   │
   ├── Batch / Lot
   │
   ▼
Warehouse
   │
   ▼
Inventory Stock
   │
   ├── Reservation
   │
   └── Inventory Ledger
          │
          ├── Receive
          ├── Issue
          ├── Transfer
          ├── Adjustment
          └── Return
```

Supporting domains:

``` text
Cycle Counting
Inventory Costing
Inventory Valuation
Replenishment
Demand Planning
ABC Analysis
Supplier
Reports
Audit
Analytics
```

------------------------------------------------------------------------

# 10. Database Strategy

Production database architecture:

``` text
Application
    │
   GORM
    │
PostgreSQL 17
    ↑
Versioned SQL migrations
```

Migration structure:

``` text
migrations/
├── 000001_create_users.up.sql
├── 000001_create_users.down.sql
├── 000002_create_products.up.sql
├── 000002_create_categories.up.sql
├── 000003_create_warehouses.up.sql
├── 000004_create_inventory.up.sql
└── ...
```

Rules:

-   Every production schema change is versioned.
-   Migration execution is part of deployment.
-   Migration changes are reviewed.
-   AutoMigrate is disabled in production.
-   GORM models remain the application data model.

------------------------------------------------------------------------

# 11. Product Management

Product fields:

``` text
id
sku
name
description
category_id
base_unit
barcode
status
minimum_stock
maximum_stock
reorder_point
safety_stock
created_at
updated_at
```

Requirements:

-   SKU is unique.
-   Product can be active or archived.
-   Product can belong to a category.
-   Product can exist across multiple warehouses.
-   Product detail must expose stock, batches, ledger, movements, and
    valuation.

------------------------------------------------------------------------

# 12. Category Management

Fields:

``` text
id
name
description
status
created_at
updated_at
```

Requirements:

-   Unique category name.
-   Product filtering.
-   Reporting grouping.
-   ABC analysis grouping.

------------------------------------------------------------------------

# 13. Warehouse Management

Fields:

``` text
id
code
name
address
status
manager_id
created_at
updated_at
```

Requirements:

-   Warehouse code is unique.
-   Warehouse has inventory visibility.
-   Warehouse supports stock transfers.
-   Warehouse supports cycle counts.
-   Warehouse dashboard shows stock health and value.

------------------------------------------------------------------------

# 14. Inventory Stock

`inventory_stocks` represents current stock state.

Fields:

``` text
id
product_id
warehouse_id
quantity
reserved_quantity
version
created_at
updated_at
```

Constraint:

``` text
UNIQUE(product_id, warehouse_id)
```

Formula:

``` text
Available Stock = On Hand - Reserved
```

Rules:

-   Quantity cannot become negative.
-   Reserved quantity cannot exceed on-hand quantity.
-   Stock updates must use transactions.
-   Concurrent updates must be protected.

------------------------------------------------------------------------

# 15. Inventory Ledger

The inventory ledger is a defining Inventra feature.

Every stock-changing operation creates a ledger record.

Example:

``` text
Opening Balance       100
Receive                +50
Issue                  -20
Transfer Out           -10
Adjustment              -5
---------------------------
Current Balance        115
```

Fields:

``` text
id
product_id
warehouse_id
batch_id
transaction_type
quantity
unit_cost
total_cost
reference_type
reference_id
reason
performed_by
created_at
```

Transaction types:

``` text
OPENING_BALANCE
RECEIVE
ISSUE
TRANSFER_IN
TRANSFER_OUT
ADJUSTMENT
RETURN
```

Ledger records should be append-oriented and auditable.

Current stock must remain consistent with ledger operations.

------------------------------------------------------------------------

# 16. Inventory Transaction Pipeline

All critical inventory operations follow:

``` text
HTTP Request
    ↓
Request ID
    ↓
Authentication
    ↓
Authorization
    ↓
Validation
    ↓
Idempotency Check
    ↓
BEGIN TRANSACTION
    ↓
SELECT ... FOR UPDATE
    ↓
Validate Current State
    ↓
Modify Stock
    ↓
Create Ledger Entry
    ↓
Create Audit Entry
    ↓
COMMIT
```

Failure:

``` text
ROLLBACK
```

No partial inventory state is allowed.

------------------------------------------------------------------------

# 17. Concurrency Control

Use PostgreSQL row-level locking where necessary.

Example:

``` sql
SELECT *
FROM inventory_stocks
WHERE product_id = ?
AND warehouse_id = ?
FOR UPDATE;
```

Required scenario:

``` text
Initial stock = 100

Request A → Issue 70
Request B → Issue 50
```

Expected:

``` text
One request succeeds.
One request fails.
Final stock = 30.
```

Never allow:

-   negative stock from race conditions
-   lost updates
-   duplicate movement records
-   partial transfers

------------------------------------------------------------------------

# 18. Idempotency

Critical endpoints:

``` text
POST /inventory/receive
POST /inventory/issue
POST /inventory/adjust
POST /inventory/transfers
```

must support:

``` http
Idempotency-Key: unique-request-key
```

Behavior:

``` text
First request
    ↓
Create transaction
    ↓
Store result

Repeated request
    ↓
Return existing result
```

Same idempotency key must not create a second inventory movement.

------------------------------------------------------------------------

# 19. Inventory Receiving

Required inputs:

``` text
Product
Warehouse
Batch
Quantity
Unit Cost
Supplier Reference
Receiving Date
Production Date
Expiry Date
```

Flow:

``` text
Receive Request
    ↓
Validate
    ↓
Create / Update Batch
    ↓
Increase Stock
    ↓
Create Ledger
    ↓
Update Valuation
    ↓
Create Audit
    ↓
Commit
```

------------------------------------------------------------------------

# 20. Inventory Issuing

Requirements:

-   Quantity \> 0.
-   Available stock is sufficient.
-   User has permission.
-   Batch selection is valid.
-   FEFO applies where expiry tracking is enabled.
-   Cost is calculated.
-   Ledger entry is created.
-   Audit entry is created.

------------------------------------------------------------------------

# 21. Stock Reservation

Reservation exists to answer:

> How much stock is actually available?

Formula:

``` text
Available = On Hand - Reserved
```

Reservation fields:

``` text
id
product_id
warehouse_id
quantity
reference_type
reference_id
status
expires_at
created_at
```

Statuses:

``` text
ACTIVE
RELEASED
CONSUMED
EXPIRED
```

Inventra does not own picking, packing, drivers, routing, or delivery.

------------------------------------------------------------------------

# 22. Warehouse Transfer

Example:

``` text
Warehouse A = 100
Warehouse B = 50

Transfer = 20

Warehouse A = 80
Warehouse B = 70
```

Atomic workflow:

``` text
BEGIN
 ↓
Lock source stock
 ↓
Lock destination stock
 ↓
Validate source quantity
 ↓
Decrease source
 ↓
Increase destination
 ↓
Create TRANSFER_OUT
 ↓
Create TRANSFER_IN
 ↓
Create Audit
 ↓
COMMIT
```

Any failure causes rollback.

------------------------------------------------------------------------

# 23. Stock Adjustment Workflow

Direct stock editing is prohibited.

Workflow:

``` text
Adjustment Request
       ↓
Reason
       ↓
Evidence / Notes
       ↓
Review
       ↓
Approval
       ↓
Ledger Entry
       ↓
Audit
```

Reasons:

``` text
DAMAGED
LOST
FOUND
COUNT_VARIANCE
DATA_CORRECTION
EXPIRATION
QUALITY_REJECT
OTHER
```

High-value adjustments require manager approval.

------------------------------------------------------------------------

# 24. Cycle Counting

Cycle counting is a major Inventra feature.

Workflow:

``` text
Count Plan
    ↓
Assignment
    ↓
Physical Count
    ↓
Compare System Quantity
    ↓
Calculate Variance
    ↓
Review
    ↓
Approval
    ↓
Adjustment
    ↓
Ledger
    ↓
Audit
```

Example:

``` text
System = 500
Physical = 487
Variance = -13
```

Store:

``` text
system_quantity
counted_quantity
variance
counter
reviewer
reason
approved_by
created_at
```

------------------------------------------------------------------------

# 25. ABC Analysis

Classify inventory by value and/or movement.

``` text
A = high-value / high-impact
B = medium-value
C = low-value
```

Use ABC classification to influence counting frequency:

``` text
A → frequent counting
B → regular counting
C → periodic counting
```

Important relationship:

``` text
ABC Analysis
    ↓
Counting Frequency
    ↓
Cycle Count
    ↓
Variance
    ↓
Adjustment
    ↓
Audit
```

------------------------------------------------------------------------

# 26. Batch / Lot Tracking

Batch fields:

``` text
id
product_id
batch_number
production_date
expiry_date
quantity
unit_cost
status
created_at
updated_at
```

Traceability:

``` text
Product
  ↓
Batch
  ↓
Warehouse Stock
  ↓
Ledger
  ↓
Transaction
  ↓
Audit
```

------------------------------------------------------------------------

# 27. Expiry Management

Expiry statuses:

``` text
NORMAL
EXPIRING_SOON
EXPIRED
```

Display:

-   SKU
-   Product
-   Batch
-   Warehouse
-   Expiry date
-   Days remaining
-   Quantity
-   Inventory value

------------------------------------------------------------------------

# 28. FEFO

For expiry-sensitive products:

> First Expired, First Out.

Example:

``` text
Batch A → expires in 5 days
Batch B → expires in 20 days

Issue 50
    ↓
Use Batch A first
```

FEFO must be deterministic and explainable.

------------------------------------------------------------------------

# 29. Inventory Costing

Support:

## FIFO

Older cost layers are consumed first.

``` text
100 units × $10
100 units × $12

Issue 120

100 × $10
20 × $12
```

## Weighted Average

``` text
100 × $10
100 × $12

Average Cost = $11
```

Track:

``` text
unit_cost
total_cost
inventory_value
COGS
```

Costing calculations must be deterministic and covered by tests.

------------------------------------------------------------------------

# 30. Inventory Valuation

Display:

-   Total inventory value
-   Value by warehouse
-   Value by product
-   Value by category
-   Value by batch

Example:

``` text
Warehouse A → $250,000
Warehouse B → $175,000
Total → $425,000
```

------------------------------------------------------------------------

# 31. Stock Aging

Classify inventory by age:

``` text
0–30 days
31–60 days
61–90 days
91–180 days
180+ days
```

Use this to identify slow-moving inventory and capital tied up in stock.

------------------------------------------------------------------------

# 32. Dead Stock Detection

Identify:

-   no movement
-   very low movement
-   excessive age
-   high inventory value

Example:

``` text
SKU: RM-028
Quantity: 500
Last Movement: 184 days ago
Value: $18,000
Status: DEAD STOCK
```

------------------------------------------------------------------------

# 33. Reorder Engine

Parameters:

``` text
Current Stock
Minimum Stock
Maximum Stock
Reorder Point
Safety Stock
Lead Time
Average Demand
```

Flow:

``` text
Current Stock
    ↓
Compare Reorder Point
    ↓
Below Threshold?
    ↓
Calculate Recommendation
```

Example:

``` text
Current Stock = 120
Reorder Point = 150
Recommended Order = 200
```

------------------------------------------------------------------------

# 34. Demand Planning

Initial methods:

-   Moving Average
-   Weighted Moving Average
-   Exponential Smoothing

Flow:

``` text
Historical Demand
    ↓
Demand Estimate
    ↓
Safety Stock
    ↓
Reorder Recommendation
```

No AI dependency is required.

------------------------------------------------------------------------

# 35. Supplier Management

Fields:

``` text
id
name
code
contact
email
phone
lead_time_days
status
created_at
updated_at
```

Supplier information supports replenishment planning.

------------------------------------------------------------------------

# 36. Purchase Replenishment

Future workflow:

``` text
Low Stock
    ↓
Recommendation
    ↓
Supplier Selection
    ↓
Purchase Order
    ↓
Expected Receipt
    ↓
Receive Inventory
    ↓
Ledger
```

Inventra's core responsibility ends at inventory receiving.

------------------------------------------------------------------------

# 37. Unit Conversion

Support deterministic conversion rules.

Example:

``` text
1 Carton = 24 Boxes
1 Box = 12 Pieces
1 Carton = 288 Pieces
```

Possible units:

``` text
Purchase Unit
Storage Unit
Consumption Unit
```

All conversions must be auditable.

------------------------------------------------------------------------

# 38. Inventory Analytics

Analytics should answer business questions.

## Required metrics

### Stock Health

-   Total SKUs
-   Available stock
-   Low-stock SKUs
-   Out-of-stock SKUs
-   Expiring inventory

### Value

-   Total inventory value
-   Value by warehouse
-   Value by category

### Movement

-   Receive volume
-   Issue volume
-   Transfer volume
-   Adjustment volume

### Efficiency

-   Inventory turnover
-   Stock aging
-   Dead stock
-   Stock variance

### Planning

-   Reorder recommendations
-   Safety-stock violations
-   Demand trends

------------------------------------------------------------------------

# 39. Reports

Required reports:

``` text
Inventory Summary
Stock Ledger
Stock Movement
Inventory Valuation
Stock Aging
Dead Stock
Low Stock
Cycle Count Variance
Batch / Expiry
Warehouse Inventory
ABC Analysis
Reorder Recommendations
```

Reports support:

-   date ranges
-   warehouse filtering
-   product filtering
-   category filtering
-   sorting
-   export

------------------------------------------------------------------------

# 40. Authentication

Authentication:

``` text
JWT HS256
Access Token
Rotating Refresh Token
bcrypt
```

Recommended:

``` text
Access Token: 10–15 minutes
Refresh Token: 7–30 days
```

Refresh token rotation:

``` text
Refresh A
   ↓
Invalidate A
   ↓
Generate B
```

Reuse detection:

``` text
Old Refresh A reused
   ↓
Detect reuse
   ↓
Revoke session/token family
   ↓
Require login
```

Never log:

-   passwords
-   JWTs
-   refresh tokens
-   secrets

------------------------------------------------------------------------

# 41. RBAC

Permissions:

``` text
product.read
product.create
product.update
product.delete

inventory.read
inventory.receive
inventory.issue
inventory.adjust
inventory.transfer

warehouse.read
warehouse.manage

report.read
report.export

user.manage
audit.read
```

Roles:

``` text
ADMIN
WAREHOUSE_MANAGER
STAFF
VIEWER
```

Authorization is always enforced on the backend.

------------------------------------------------------------------------

# 42. Audit Logging

Audit fields:

``` text
id
user_id
action
entity_type
entity_id
before_data
after_data
reason
ip_address
user_agent
request_id
created_at
```

Important events:

``` text
USER_CREATED
USER_UPDATED
ROLE_CHANGED

PRODUCT_CREATED
PRODUCT_UPDATED

INVENTORY_RECEIVED
INVENTORY_ISSUED
INVENTORY_ADJUSTED
INVENTORY_TRANSFERRED

WAREHOUSE_CREATED
WAREHOUSE_UPDATED

CYCLE_COUNT_APPROVED
REORDER_CREATED
```

Audit records are append-oriented and protected from ordinary
modification.

------------------------------------------------------------------------

# 43. API Design

Base:

``` text
/api/v1/
```

Products:

``` text
GET    /api/v1/products
POST   /api/v1/products
GET    /api/v1/products/:id
PATCH  /api/v1/products/:id
DELETE /api/v1/products/:id
```

Inventory:

``` text
GET  /api/v1/inventory
GET  /api/v1/inventory/:id
POST /api/v1/inventory/receive
POST /api/v1/inventory/issue
POST /api/v1/inventory/adjust
POST /api/v1/inventory/transfer
```

Cycle counting:

``` text
GET  /api/v1/cycle-counts
POST /api/v1/cycle-counts
POST /api/v1/cycle-counts/:id/count
POST /api/v1/cycle-counts/:id/approve
```

Analytics:

``` text
GET /api/v1/analytics/inventory-value
GET /api/v1/analytics/stock-aging
GET /api/v1/analytics/dead-stock
GET /api/v1/analytics/abc
GET /api/v1/analytics/turnover
```

------------------------------------------------------------------------

# 44. API Response Standard

Success:

``` json
{
  "data": {},
  "meta": {}
}
```

Error:

``` json
{
  "error": {
    "code": "INSUFFICIENT_STOCK",
    "message": "Insufficient stock for this operation."
  }
}
```

Stable error codes must be used by the frontend.

------------------------------------------------------------------------

# 45. Pagination

All large collections use:

``` text
?page=1
&limit=20
&search=
&sort=
&order=
```

Response:

``` json
{
  "data": [],
  "meta": {
    "page": 1,
    "limit": 20,
    "total": 157,
    "total_pages": 8
  }
}
```

------------------------------------------------------------------------

# 46. Frontend Architecture

Recommended:

``` text
src/
├── app/
├── components/
├── features/
│   ├── auth/
│   ├── products/
│   ├── inventory/
│   ├── warehouses/
│   ├── cycle-counts/
│   ├── costing/
│   ├── batches/
│   ├── replenishment/
│   ├── analytics/
│   └── reports/
├── api/
├── hooks/
├── schemas/
├── types/
└── lib/
```

State rules:

-   TanStack Query owns server state.
-   React local state owns local UI state.
-   Do not introduce Redux/Zustand without a real requirement.

------------------------------------------------------------------------

# 47. UI/UX Design Direction

## Visual Character

-   Modern enterprise SaaS
-   Desktop-first
-   Clean
-   Dense but readable
-   Data-focused
-   Soft shadows
-   Rounded cards
-   Clear status indicators
-   Minimal decoration

Primary visual hierarchy:

``` text
Decision
  ↓
Action
  ↓
Data
  ↓
Details
```

The interface should prioritize operational decisions over decoration.

------------------------------------------------------------------------

# 48. Navigation

Primary sidebar:

``` text
OVERVIEW
  Dashboard

INVENTORY
  Products
  Stock Overview
  Inventory Ledger
  Batches & Lots
  Stock Movements

WAREHOUSE
  Warehouses
  Transfers
  Cycle Counting
  Adjustments

PLANNING
  Reorder
  Suppliers
  Demand Planning

INTELLIGENCE
  Analytics
  Inventory Valuation
  Stock Aging
  Dead Stock
  ABC Analysis

REPORTS
  Reports

ADMINISTRATION
  Users
  Roles & Permissions
  Audit Log
  Settings
```

Do not overload the navigation with every possible feature.

------------------------------------------------------------------------

# 49. Dashboard UX

The dashboard must answer:

> What needs my attention right now?

Top KPIs:

``` text
Inventory Value
Available Stock
Low Stock Items
Expiring Soon
Count Variance
```

Main areas:

``` text
Inventory Value Trend
Stock by Status
Requires Attention
Top Low Stock Items
Recent Movements
Quick Actions
```

Every warning must be actionable.

Bad:

``` text
13 Expiring
```

Better:

``` text
13 batches expire within 30 days → View
```

------------------------------------------------------------------------

# 50. Dashboard Wireframe

``` text
┌─────────────────────────────────────────────────────────────┐
│ INVENTRA                                  Search   Bell User │
├──────────────┬──────────────────────────────────────────────┤
│              │ Dashboard                                    │
│ Dashboard    │                                              │
│              │ [Inventory Value] [Available] [Low Stock]   │
│ Inventory    │ [Expiring] [Count Variance]                 │
│ Products     │                                              │
│ Stock        │ ┌─────────────────┐ ┌──────────────────────┐ │
│ Ledger       │ │ Value Trend     │ │ Stock by Status      │ │
│ Batches      │ │                 │ │                      │ │
│              │ └─────────────────┘ └──────────────────────┘ │
│ Warehouse    │                                              │
│ Transfers    │ ┌──────────────────────────────────────────┐ │
│ Counting     │ │ Requires Attention                       │ │
│              │ │ Low Stock       → View                   │ │
│ Planning     │ │ Expiring        → View                   │ │
│ Reorder      │ │ Count Variance → Review                 │ │
│ Suppliers    │ │ Reorder         → Review                 │ │
│              │ └──────────────────────────────────────────┘ │
│ Intelligence │                                              │
│ Analytics    │ ┌──────────────────┐ ┌─────────────────────┐ │
│ Valuation    │ │ Low Stock Items  │ │ Recent Movements    │ │
│ Aging        │ │                  │ │                     │ │
│ Dead Stock   │ └──────────────────┘ └─────────────────────┘ │
│ ABC          │                                              │
└──────────────┴──────────────────────────────────────────────┘
```

------------------------------------------------------------------------

# 51. Stock Overview UX

Primary table:

``` text
SKU
Product
Warehouse
On Hand
Reserved
Available
Unit
Value
Status
```

Filters:

``` text
Warehouse
Category
Status
Search
```

Table requirements:

-   Sticky header
-   Sorting
-   Filtering
-   Pagination
-   Column visibility
-   Export
-   Row selection where needed
-   Loading skeleton
-   Empty state
-   Error state

------------------------------------------------------------------------

# 52. Product Detail UX

Product detail is a command center.

Header:

``` text
RM-001
Premium Fragrance Base

[Edit] [Adjust Stock] [Reserve] [More]
```

Tabs:

``` text
Overview
Stock by Warehouse
Ledger
Batches
Movements
Specifications
```

Summary:

``` text
Current Stock
Inventory Value
Status
Reorder Point
```

Quick actions:

``` text
Receive Stock
Issue Stock
Transfer Stock
Create Reservation
View Ledger
```

------------------------------------------------------------------------

# 53. Ledger UX

Ledger should visually resemble a financial record.

Columns:

``` text
Date
Type
Reference
Warehouse
Quantity
Balance
Unit Cost
Created By
```

Filters:

``` text
Product
Warehouse
Transaction Type
Date Range
```

Clicking a row opens transaction details and audit information.

------------------------------------------------------------------------

# 54. Receive Stock UX

Guided form:

``` text
Product
Warehouse
Batch
Quantity
Unit Cost
Production Date
Expiry Date
Supplier Reference
```

Show live summary:

``` text
Quantity
Unit Cost
Total Cost
```

Primary action:

``` text
Receive Inventory
```

------------------------------------------------------------------------

# 55. Transfer UX

Use a source-to-destination visual flow:

``` text
FROM
Jakarta Warehouse
Available: 2,100

        ↓ Transfer

TO
Semarang Warehouse
Current: 1,520
```

Show:

-   Product
-   Quantity
-   Reason
-   Current source stock
-   Destination stock
-   Resulting balances

------------------------------------------------------------------------

# 56. Cycle Counting UX

Cycle counting should optimize for speed.

Display:

``` text
Progress: 82 / 100
SKU
Product
Warehouse
System Quantity
Physical Count
Variance
Reason
```

Primary action:

``` text
Save & Next
```

Use keyboard-friendly interactions where practical.

------------------------------------------------------------------------

# 57. Adjustment Approval UX

Show before/after state:

``` text
System Quantity: 500
Counted Quantity: 487
Adjustment: -13

Reason: Count Variance

After approval:
Stock → 487
Ledger → ADJUSTMENT -13
Audit → Created
```

Actions:

``` text
Reject
Approve
```

Require explicit confirmation for destructive or high-impact actions.

------------------------------------------------------------------------

# 58. Batch & Expiry UX

Table:

``` text
Batch
Product
Warehouse
Quantity
Expiry
Days Remaining
Status
```

Statuses:

``` text
Healthy
Expiring Soon
Expired
```

Provide filtering by:

-   Warehouse
-   Product
-   Expiry range
-   Status

------------------------------------------------------------------------

# 59. Reorder UX

Reorder page is an action queue.

Each recommendation shows:

``` text
Product
Current Stock
Reorder Point
Recommended Quantity
Supplier
Lead Time
Estimated Need
```

Primary action:

``` text
Create Purchase Order
```

------------------------------------------------------------------------

# 60. Analytics UX

Separate operational dashboard from analytical dashboard.

Analytics should include:

``` text
Inventory Value
Turnover
Stock Aging
Dead Stock
Stock Variance
ABC Analysis
Demand Trend
Reorder Recommendations
```

Charts must support filtering by:

-   Date range
-   Warehouse
-   Category
-   Product

------------------------------------------------------------------------

# 61. Design System

Typography:

``` text
Inter
```

Suggested hierarchy:

``` text
Page title: 24–28px
Section heading: 18–20px
Body: 14–16px
Metadata: 12–13px
```

Use shadcn/ui as the component foundation.

Core components:

-   Table
-   Dialog
-   Sheet
-   Tabs
-   Badge
-   Dropdown
-   Command
-   Tooltip
-   Toast
-   Progress
-   Calendar
-   Date Picker
-   Select
-   Form
-   Alert

------------------------------------------------------------------------

# 62. Color Semantics

Use colors intentionally.

``` text
Green  = healthy / success
Yellow = warning / expiring
Red    = critical / error
Blue   = informational / primary action
Gray   = neutral
```

Do not rely on color alone.

Use:

``` text
icon + text + color
```

for important statuses.

------------------------------------------------------------------------

# 63. UX Rules

## Rule 1 --- Decision First

The user should see what requires attention before deep analytics.

## Rule 2 --- Traceability First

Every stock number should be explainable:

``` text
Current Stock
    ↓
Ledger
    ↓
Transaction
    ↓
User
    ↓
Audit
```

## Rule 3 --- Action Proximity

Users should not need five screens to resolve a warning.

## Rule 4 --- Progressive Disclosure

Summary first.

Details on demand.

## Rule 5 --- Prevent Mistakes

Use:

-   validation
-   confirmation
-   permission checks
-   previews
-   warnings
-   transaction summaries

------------------------------------------------------------------------

# 64. Global Search

Top bar:

``` text
Search products, SKU, batch, warehouse...
```

Shortcut:

``` text
Ctrl + K
```

Search results grouped by:

``` text
Products
Batches
Transactions
Warehouses
```

------------------------------------------------------------------------

# 65. Notification Center

Only show useful operational notifications:

``` text
4 products below reorder point
13 batches expire within 30 days
8 cycle counts require review
3 adjustments pending
```

Do not create social-style notification noise.

------------------------------------------------------------------------

# 66. Responsive Rules

Desktop is the primary environment because inventory management requires
large tables.

## Desktop

-   Full sidebar
-   Full tables
-   Multi-column dashboards

## Tablet

-   Collapsed sidebar
-   Reduced columns
-   Horizontal table scrolling

## Mobile

Prioritize:

-   Dashboard
-   Stock lookup
-   Product details
-   Cycle counting
-   Approval actions

Complex reports remain desktop-oriented.

------------------------------------------------------------------------

# 67. Error Handling

Stable error codes:

``` text
INVALID_REQUEST
UNAUTHORIZED
FORBIDDEN
NOT_FOUND
CONFLICT
INSUFFICIENT_STOCK
DUPLICATE_REQUEST
VALIDATION_FAILED
INTERNAL_ERROR
```

Frontend should use error codes rather than parsing arbitrary error
messages.

Never expose:

-   SQL errors
-   stack traces
-   database credentials
-   filesystem paths
-   secrets

------------------------------------------------------------------------

# 68. Observability

Continue Zap structured logging.

Required fields:

``` text
timestamp
level
request_id
user_id
method
route
status
duration
error
```

Inventory operations may additionally include:

``` text
product_id
warehouse_id
transaction_id
```

Never log sensitive credentials.

Health endpoints:

``` text
GET /health
GET /ready
```

Future optional observability:

``` text
OpenTelemetry
Prometheus
Grafana
```

Only after the core system is stable.

------------------------------------------------------------------------

# 69. Testing Strategy

## Unit Tests

Test:

-   Inventory business rules
-   Cost calculations
-   Reorder calculations
-   FEFO logic
-   ABC classification
-   Validation
-   Authorization

## Integration Tests

Use real PostgreSQL.

Test:

-   Transactions
-   Rollbacks
-   Foreign keys
-   Unique constraints
-   Row locking
-   Inventory updates
-   Transfers
-   Costing

## Concurrency Tests

Mandatory scenario:

``` text
Stock = 100

Issue 70
Issue 50 concurrently

Expected:
one success
one failure
final stock = 30
```

## E2E Workflow

``` text
Login
 ↓
Create Product
 ↓
Create Warehouse
 ↓
Receive Batch
 ↓
Reserve Stock
 ↓
Issue Stock
 ↓
Transfer Stock
 ↓
Cycle Count
 ↓
Approve Variance
 ↓
View Ledger
 ↓
View Analytics
```

------------------------------------------------------------------------

# 70. CI/CD

Pipeline:

``` text
Checkout
 ↓
Dependency Validation
 ↓
Lint
 ↓
Unit Tests
 ↓
Integration Tests
 ↓
Coverage
 ↓
Build
 ↓
Security Checks
 ↓
Docker Build
```

Coverage target:

``` text
>= 80%
```

Coverage is not a substitute for critical business tests.

------------------------------------------------------------------------

# 71. Performance Targets

Initial targets:

``` text
Typical API request: < 300ms
Typical database query: < 100ms
```

Exceptions are allowed for intentionally complex analytics/report
queries.

Large collections must be paginated.

Index common access patterns:

-   SKU
-   Product ID
-   Warehouse ID
-   Batch number
-   Transaction timestamp
-   Audit timestamp
-   Foreign keys
-   Reorder state

Do not add indexes without a query/use-case reason.

------------------------------------------------------------------------

# 72. Security Requirements

Implement:

-   bcrypt password hashing
-   short-lived access tokens
-   refresh-token rotation
-   refresh-token reuse detection
-   RBAC
-   backend permission enforcement
-   input validation
-   rate limiting
-   CORS configuration
-   secure HTTP headers
-   parameterized database access
-   secret management
-   authentication failure protection

Frontend authorization is never a security boundary.

------------------------------------------------------------------------

# 73. P0 --- Mandatory Foundation

P0 must be completed first.

### Architecture

-   [ ] Modular monolith boundaries
-   [ ] Versioned production migrations
-   [ ] Database constraints
-   [ ] Explicit transactions

### Inventory correctness

-   [ ] Inventory ledger
-   [ ] Row-level locking
-   [ ] Atomic transfers
-   [ ] Negative-stock prevention
-   [ ] Idempotency

### Security

-   [ ] JWT rotation
-   [ ] Refresh-token reuse detection
-   [ ] RBAC
-   [ ] Permission middleware
-   [ ] Audit logging

------------------------------------------------------------------------

# 74. P1 --- Product Differentiation

P1 makes Inventra meaningfully different from Vayra.

-   [ ] Cycle counting
-   [ ] Adjustment approval
-   [ ] Inventory costing
-   [ ] FIFO
-   [ ] Weighted average
-   [ ] Batch / lot tracking
-   [ ] Expiry management
-   [ ] FEFO
-   [ ] Inventory valuation
-   [ ] Stock aging
-   [ ] Dead-stock detection
-   [ ] Reorder engine
-   [ ] Inventory analytics
-   [ ] Integration tests
-   [ ] Concurrency tests
-   [ ] E2E tests
-   [ ] Request IDs
-   [ ] Health checks
-   [ ] API response standards
-   [ ] Pagination

------------------------------------------------------------------------

# 75. P2 --- Advanced Inventory Intelligence

-   [ ] ABC analysis
-   [ ] Demand planning
-   [ ] Moving average
-   [ ] Weighted moving average
-   [ ] Exponential smoothing
-   [ ] Unit conversion
-   [ ] Supplier management
-   [ ] Purchase replenishment
-   [ ] Advanced reports
-   [ ] OpenTelemetry evaluation
-   [ ] Metrics evaluation

------------------------------------------------------------------------

# 76. Deferred Infrastructure

Do not add unless justified:

``` text
Redis
Kafka
RabbitMQ
NATS
Microservices
gRPC
Elasticsearch
Kubernetes
Service Mesh
```

Possible future Redis use:

-   cache
-   rate limiting
-   temporary data

PostgreSQL remains authoritative.

------------------------------------------------------------------------

# 77. Recommended Development Sequence

## Phase 1 --- Foundation

``` text
Architecture
 ↓
Migrations
 ↓
Database Constraints
 ↓
RBAC
 ↓
Audit
```

## Phase 2 --- Inventory Core

``` text
Inventory Stock
 ↓
Ledger
 ↓
Receive
 ↓
Issue
 ↓
Transfer
 ↓
Reservation
```

## Phase 3 --- Correctness

``` text
Transactions
 ↓
Row Locking
 ↓
Idempotency
 ↓
Concurrency Tests
```

## Phase 4 --- Inventory Control

``` text
Cycle Counting
 ↓
Variance
 ↓
Approval
 ↓
Adjustment
```

## Phase 5 --- Inventory Intelligence

``` text
Costing
 ↓
Valuation
 ↓
Batch
 ↓
Expiry
 ↓
FEFO
 ↓
Aging
 ↓
Dead Stock
```

## Phase 6 --- Planning

``` text
Reorder
 ↓
ABC
 ↓
Demand Planning
 ↓
Supplier
 ↓
Purchase Replenishment
```

## Phase 7 --- Production Quality

``` text
Integration Tests
 ↓
E2E
 ↓
CI/CD
 ↓
Observability
 ↓
Documentation
```

------------------------------------------------------------------------

# 78. Portfolio Positioning

Inventra should demonstrate:

## Transactional Systems

``` text
PostgreSQL
Transactions
Row Locks
Concurrency
Idempotency
```

## Enterprise Domain Modeling

``` text
Inventory Ledger
Cycle Counting
Costing
Batch Tracking
Expiry
Replenishment
```

## Business Intelligence

``` text
Inventory Valuation
ABC Analysis
Stock Aging
Dead Stock
Demand Planning
```

## Governance

``` text
RBAC
Audit
Approval Workflow
Traceability
```

------------------------------------------------------------------------

# 79. Inventra vs Vayra Boundary

Inventra owns:

``` text
Inventory
Warehouses
Stock
Ledger
Reservations
Costing
Batches
Expiry
Counting
Valuation
Replenishment
Planning
```

Vayra owns:

``` text
Orders
Fulfillment
Picking
Packing
Shipment
Drivers
Routes
GPS
Delivery
Real-time Logistics
Event-driven Orchestration
```

Future integration may allow Vayra to consume Inventra's inventory
availability, but Inventra must not absorb Vayra's logistics domain.

------------------------------------------------------------------------

# 80. Final System Design

``` text
                         INVENTRA
                            │
          ┌─────────────────┼─────────────────┐
          ▼                 ▼                 ▼
       Catalog           Warehouse         Inventory
                                              │
                              ┌───────────────┼───────────────┐
                              ▼               ▼               ▼
                           Ledger        Reservation        Batch
                              │               │               │
                    ┌─────────┼─────────┐     │            Expiry
                    ▼         ▼         ▼     │               │
                 Receive    Issue    Transfer │              FEFO
                    │         │         │      │
                    └─────────┼─────────┘      │
                              ▼                │
                         Stock State ◄─────────┘
                              │
                              ▼
                       Cycle Counting
                              │
                              ▼
                           Variance
                              │
                              ▼
                           Approval
                              │
                              ▼
                          Adjustment
                              │
                              ▼
                            Audit

                              │
                              ▼
                    INVENTORY INTELLIGENCE
                              │
             ┌────────────────┼─────────────────┐
             ▼                ▼                 ▼
          Costing         Reordering        Analytics
             │                │                 │
             ▼                ▼                 ▼
         Valuation       ABC Analysis       Aging
                                             Dead Stock
                                             Turnover
                                             Variance

                              │
                              ▼
                       Demand Planning
                              │
                              ▼
                          Suppliers
                              │
                              ▼
                     Purchase Replenishment
```

------------------------------------------------------------------------

# 81. Core UI Structure

``` text
INVENTRA
│
├── Dashboard
│
├── Inventory
│   ├── Products
│   ├── Stock Overview
│   ├── Inventory Ledger
│   ├── Batches & Lots
│   └── Stock Movements
│
├── Warehouse
│   ├── Warehouses
│   ├── Transfers
│   ├── Cycle Counting
│   └── Adjustments
│
├── Planning
│   ├── Reorder
│   ├── Suppliers
│   └── Demand Planning
│
├── Intelligence
│   ├── Analytics
│   ├── Inventory Valuation
│   ├── Stock Aging
│   ├── Dead Stock
│   └── ABC Analysis
│
├── Reports
│
└── Administration
    ├── Users
    ├── Roles & Permissions
    ├── Audit Log
    └── Settings
```

------------------------------------------------------------------------

# 82. Core UI Screens

The highest-priority screens are:

1.  Dashboard
2.  Stock Overview
3.  Product Detail
4.  Inventory Ledger
5.  Cycle Counting
6.  Adjustment Approval
7.  Batch & Expiry
8.  Reorder Recommendations
9.  Inventory Analytics

These screens should receive the highest visual-polish priority.

------------------------------------------------------------------------

# 83. Core UI Rules

### Tables

-   Sticky headers
-   Sorting
-   Filtering
-   Pagination
-   Column visibility
-   Export
-   Loading states
-   Empty states
-   Error states

### Statuses

Use:

``` text
Healthy
Low Stock
Out of Stock
Expiring Soon
Expired
Pending
Approved
Rejected
In Progress
Completed
```

Use icon + text + color.

### Confirmation

High-impact operations show:

-   before state
-   after state
-   difference
-   reason
-   resulting ledger entry
-   audit consequence

------------------------------------------------------------------------

# 84. Primary User Flows

## Receive

``` text
Dashboard
 ↓
Receive Inventory
 ↓
Product
 ↓
Warehouse
 ↓
Batch
 ↓
Quantity + Cost
 ↓
Review
 ↓
Confirm
 ↓
Ledger
 ↓
Updated Stock
```

## Transfer

``` text
Stock
 ↓
Transfer
 ↓
Source
 ↓
Destination
 ↓
Quantity
 ↓
Review
 ↓
Confirm
 ↓
Atomic Transfer
 ↓
Ledger
```

## Cycle Count

``` text
Cycle Counting
 ↓
Count
 ↓
Physical Quantity
 ↓
Variance
 ↓
Submit
 ↓
Manager Review
 ↓
Approve
 ↓
Adjustment
 ↓
Ledger
 ↓
Audit
```

## Replenishment

``` text
Dashboard
 ↓
Low Stock
 ↓
Reorder Recommendation
 ↓
Review Demand
 ↓
Select Supplier
 ↓
Create Purchase Order
```

------------------------------------------------------------------------

# 85. Definition of Done

Inventra is considered production-ready for portfolio demonstration
when:

1.  Inventory cannot become corrupted through concurrent requests.
2.  Every stock change is traceable through the ledger.
3.  Transfers are atomic.
4.  Duplicate requests cannot create duplicate movements.
5.  Sensitive operations are auditable.
6.  Adjustments are controlled by permissions and approvals.
7.  Physical inventory differences can be reconciled through cycle
    counting.
8.  Inventory value can be calculated using explicit costing rules.
9.  Batches and expiry can be traced.
10. FEFO works for expiry-sensitive inventory.
11. Dead, aging, and low-stock inventory can be identified.
12. Replenishment recommendations can be generated.
13. Inventory analytics provide actionable insights.
14. Critical flows have unit, integration, concurrency, and E2E tests.
15. Production schema changes are version-controlled.
16. Request IDs make operational failures traceable.
17. The UI makes operational actions obvious.
18. The project remains a modular monolith.
19. Inventra remains clearly distinct from Vayra.

------------------------------------------------------------------------

# 86. Final Engineering Principle

The project should prioritize:

``` text
Correctness
    ↓
Maintainability
    ↓
Auditability
    ↓
Business Utility
    ↓
Observability
    ↓
Scalability
```

Do not add infrastructure merely because it appears in an enterprise
architecture diagram.

The strongest version of Inventra is not the one with the most
technologies.

It is the one where the inventory domain is modeled deeply enough that
every important number has a defensible explanation.

------------------------------------------------------------------------

# 87. Final Product Statement

> **Inventra is a production-oriented inventory control and intelligence
> platform built with Go, React, and PostgreSQL, providing transactional
> stock management, immutable inventory traceability, multi-warehouse
> control, costing, batch and expiry management, cycle counting,
> replenishment planning, and inventory analytics through a clean
> modular-monolith architecture.**
