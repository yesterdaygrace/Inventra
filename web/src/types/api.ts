export type Role = "ADMIN" | "STAFF";

export interface ApiEnvelope<T> {
  success: boolean;
  message?: string;
  data: T;
  pagination?: Pagination;
}

export interface Pagination {
  page: number;
  per_page: number;
  total: number;
  total_pages: number;
}

export interface ApiErrorPayload {
  success: boolean;
  message: string;
}

// ---------- Auth ----------

export interface User {
  id: string;
  name: string;
  email: string;
  role: Role;
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

export interface AuthSession {
  access_token: string;
  refresh_token: string;
  token_type: "Bearer";
  expires_in: number;
  user: User;
}

export interface RefreshResponse {
  access_token: string;
  refresh_token: string;
  token_type: "Bearer";
  expires_in: number;
}

export interface LoginRequest {
  email: string;
  password: string;
}

export interface RegisterRequest {
  name: string;
  email: string;
  password: string;
}

// ---------- Product ----------

export interface Product {
  id: string;
  name: string;
  sku: string;
  description?: string;
  price: number;
  category_id: string;
  category_name?: string;
  low_stock_threshold: number;
  is_archived: boolean;
  stock_quantity?: number;
  is_low_stock?: boolean;
  created_at: string;
  updated_at: string;
}

export interface ProductInput {
  name: string;
  sku: string;
  description?: string;
  price: number;
  category_id: string;
  low_stock_threshold?: number;
}

export interface ProductQuery {
  page?: number;
  per_page?: number;
  q?: string;
  category_id?: string;
  low_stock?: boolean;
  is_archived?: boolean;
  sort?: string;
}

// ---------- Category ----------

export interface Category {
  id: string;
  name: string;
  description?: string;
  is_active?: boolean;
  product_count?: number;
  created_at: string;
  updated_at: string;
}

export interface CategoryInput {
  name: string;
  description?: string;
}

// ---------- Inventory ----------

export interface InventoryItem {
  product_id: string;
  product_sku: string;
  product_name: string;
  quantity: number;
  updated_at: string;
}

export interface Transaction {
  id: string;
  product_id: string;
  product_sku: string;
  product_name: string;
  type: "IN" | "OUT";
  quantity: number;
  unit_cost?: number;
  note?: string;
  user_id: string;
  created_at: string;
}

export interface StockMovementRequest {
  product_id: string;
  quantity: number;
  unit_cost?: number;
  note?: string;
}

export interface StockMovementResponse {
  product_id: string;
  quantity: number;
  updated_at: string;
}

// ---------- Dashboard ----------

export interface WarehouseHealth {
  healthy: number;
  low: number;
  critical: number;
}

export interface LowStockItem {
  product_id: string;
  sku: string;
  name: string;
  quantity: number;
  low_stock_threshold: number;
}

export interface DashboardSummary {
  total_products: number;
  total_categories: number;
  inventory_value: number;
  low_stock_count: number;
  pending_restock: number;
  warehouse_health: WarehouseHealth;
  recent_activities: Activity[];
  low_stock_items: LowStockItem[];
}

export interface Activity {
  id: string;
  user_id: string;
  user_name: string;
  action: string;
  entity_type: string;
  entity_id?: string;
  details?: string;
  ip?: string;
  created_at: string;
}

export interface ChartDataset {
  label: string;
  data: number[];
}

export interface ChartData {
  labels: string[];
  datasets: ChartDataset[];
}

// ---------- Users (admin) ----------

export interface UserInput {
  name: string;
  email: string;
  is_active?: boolean;
}

export interface RoleUpdateRequest {
  role: Role;
}

// ---------- Reports ----------

export interface CategorySummary {
  name: string;
  product_count: number;
  total_qty: number;
  total_value: number;
}

export interface ReportLowStockItem {
  product_id: string;
  sku: string;
  name: string;
  category: string;
  quantity: number;
  threshold: number;
  value: number;
}

export interface StockSummary {
  categories: CategorySummary[];
  low_stock: ReportLowStockItem[];
  total_products: number;
  total_value: number;
}

// ---------- Export (helpers) ----------

export interface ListResult<T> {
  items: T[];
  pagination: Pagination;
}