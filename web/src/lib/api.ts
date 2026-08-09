import type {
  Activity,
  ApiEnvelope,
  AuthSession,
  Category,
  CategoryInput,
  ChartData,
  DashboardSummary,
  InventoryItem,
  ListResult,
  LoginRequest,
  Product,
  ProductInput,
  ProductQuery,
  RefreshResponse,
  RegisterRequest,
  StockMovementRequest,
  StockMovementResponse,
  StockSummary,
  Transaction,
  User,
  UserInput,
  RoleUpdateRequest,
} from "@/types/api";

const API_BASE = "/api/v1";

export class ApiError extends Error {
  status: number;
  constructor(status: number, message: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
  }
}

export function isApiError(err: unknown): err is ApiError {
  return err instanceof ApiError;
}

interface StoredAuth {
  accessToken: string;
  refreshToken: string;
  expiresAt: number;
}

const STORAGE_KEY = "inventra.auth";

export function loadStoredAuth(): StoredAuth | null {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return null;
    return JSON.parse(raw) as StoredAuth;
  } catch {
    return null;
  }
}

export function storeAuth(session: { access_token: string; refresh_token: string; expires_in: number }) {
  const stored: StoredAuth = {
    accessToken: session.access_token,
    refreshToken: session.refresh_token,
    expiresAt: Date.now() + session.expires_in * 1000,
  };
  localStorage.setItem(STORAGE_KEY, JSON.stringify(stored));
}

export function clearStoredAuth() {
  localStorage.removeItem(STORAGE_KEY);
}

let refreshPromise: Promise<string> | null = null;

export function emitUnauthorized() {
  window.dispatchEvent(new CustomEvent("inventra:unauthorized"));
}

async function refreshAccessToken(): Promise<string> {
  if (refreshPromise) return refreshPromise;

  const auth = loadStoredAuth();
  if (!auth?.refreshToken) {
    emitUnauthorized();
    throw new ApiError(401, "Session expired");
  }

  refreshPromise = (async () => {
    try {
      const res = await fetch(`${API_BASE}/auth/refresh`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ refresh_token: auth.refreshToken }),
      });
      if (!res.ok) {
        clearStoredAuth();
        emitUnauthorized();
        throw new ApiError(401, "Session expired");
      }
      const body = (await res.json()) as ApiEnvelope<RefreshResponse>;
      storeAuth(body.data);
      return body.data.access_token;
    } finally {
      refreshPromise = null;
    }
  })();

  return refreshPromise;
}

async function fetchEnvelope<T>(path: string, options: RequestInit = {}): Promise<ApiEnvelope<T>> {
  const auth = loadStoredAuth();
  const headers = new Headers(options.headers);
  if (options.body && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }
  if (auth?.accessToken) {
    headers.set("Authorization", `Bearer ${auth.accessToken}`);
  }

  let res = await fetch(`${API_BASE}${path}`, { ...options, headers });

  if (res.status === 401 && auth?.refreshToken && !path.startsWith("/auth/")) {
    const token = await refreshAccessToken();
    headers.set("Authorization", `Bearer ${token}`);
    res = await fetch(`${API_BASE}${path}`, { ...options, headers });
  }

  const isJson = res.headers.get("content-type")?.includes("application/json");

  if (!res.ok) {
    let message = res.statusText || "Request failed";
    if (isJson) {
      try {
        const body = (await res.json()) as ApiEnvelope<unknown>;
        if (body?.message) message = body.message;
      } catch {
        // fall through to default message
      }
    }
    throw new ApiError(res.status, message);
  }

  if (res.status === 204) return { success: true, data: undefined as unknown as T };

  if (isJson) {
    return (await res.json()) as ApiEnvelope<T>;
  }

  return { success: true, data: (await res.text()) as unknown as T };
}

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const body = await fetchEnvelope<T>(path, options);
  return body.data;
}

async function requestList<T>(path: string, options: RequestInit = {}): Promise<ListResult<T>> {
  const body = await fetchEnvelope<T[]>(path, options);
  return {
    items: body.data ?? [],
    pagination: body.pagination ?? { page: 1, per_page: 0, total: 0, total_pages: 0 },
  };
}

export function buildQuery<T extends object>(params: T): string {
  const sp = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== "") {
      sp.set(key, String(value));
    }
  }
  const qs = sp.toString();
  return qs ? `?${qs}` : "";
}

// downloadCsv fetches an authenticated CSV endpoint and saves the response as
// a file in the browser. Handles access-token refresh on 401 like fetchEnvelope.
export async function downloadCsv(path: string, filename: string): Promise<void> {
  const auth = loadStoredAuth();
  const headers = new Headers();
  if (auth?.accessToken) headers.set("Authorization", `Bearer ${auth.accessToken}`);

  let res = await fetch(`${API_BASE}${path}`, { headers });
  if (res.status === 401 && auth?.refreshToken && !path.startsWith("/auth/")) {
    const token = await refreshAccessToken();
    headers.set("Authorization", `Bearer ${token}`);
    res = await fetch(`${API_BASE}${path}`, { headers });
  }

  if (!res.ok) {
    let message = res.statusText || "Export failed";
    if (res.headers.get("content-type")?.includes("application/json")) {
      try {
        const body = (await res.json()) as ApiEnvelope<unknown>;
        if (body?.message) message = body.message;
      } catch {
        // fall through to default message
      }
    }
    throw new ApiError(res.status, message);
  }

  const blob = await res.blob();
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(url);
}

// ---------- Auth API ----------

export const authApi = {
  login: (input: LoginRequest) =>
    request<AuthSession>("/auth/login", { method: "POST", body: JSON.stringify(input) }),

  register: (input: RegisterRequest) =>
    request<User>("/auth/register", { method: "POST", body: JSON.stringify(input) }),

  demo: () => request<AuthSession>("/auth/demo", { method: "POST" }),

  logout: (refreshToken: string) =>
    request<{ success: boolean }>("/auth/logout", {
      method: "POST",
      body: JSON.stringify({ refresh_token: refreshToken }),
    }),

  me: () => request<User>("/auth/me"),

  changePassword: (oldPassword: string, newPassword: string) =>
    request<{ success: boolean }>("/auth/change-password", {
      method: "POST",
      body: JSON.stringify({ old_password: oldPassword, new_password: newPassword }),
    }),

  updateProfile: (input: { name: string; email: string; old_password?: string }) =>
    request<User>("/auth/profile", { method: "PUT", body: JSON.stringify(input) }),
};

// ---------- Dashboard API ----------

export const dashboardApi = {
  summary: () => request<DashboardSummary>("/dashboard/summary"),

  activity: (page = 1, perPage = 20) =>
    request<Activity[]>(`/dashboard/activity${buildQuery({ page, per_page: perPage })}`),

  movement: () => request<ChartData>("/dashboard/inventory-movement"),
  categoryDistribution: () => request<ChartData>("/dashboard/category-distribution"),
  topSelling: () => request<ChartData>("/dashboard/top-selling"),
};

// ---------- Product API ----------

export const productApi = {
  list: (query: ProductQuery = {}) => requestList<Product>(`/products${buildQuery(query)}`),

  get: (id: string) => request<Product>(`/products/${id}`),

  create: (input: ProductInput) =>
    request<Product>("/products", { method: "POST", body: JSON.stringify(input) }),

  update: (id: string, input: Partial<ProductInput>) =>
    request<Product>(`/products/${id}`, { method: "PUT", body: JSON.stringify(input) }),

  archive: (id: string) =>
    request<{ success: boolean }>(`/products/${id}`, { method: "DELETE" }),

  exportCsv: () =>
    downloadCsv("/products/export", `products-${new Date().toISOString().slice(0, 10)}.csv`),
};

// ---------- Category API ----------

export const categoryApi = {
  list: (params: { page?: number; per_page?: number; name?: string; is_active?: boolean } = {}) =>
    requestList<Category>(`/categories${buildQuery(params)}`),

  get: (id: string) => request<Category>(`/categories/${id}`),

  create: (input: CategoryInput) =>
    request<Category>("/categories", { method: "POST", body: JSON.stringify(input) }),

  update: (id: string, input: Partial<CategoryInput & { is_active: boolean }>) =>
    request<Category>(`/categories/${id}`, { method: "PUT", body: JSON.stringify(input) }),

  deactivate: (id: string) =>
    request<{ success: boolean }>(`/categories/${id}`, { method: "DELETE" }),

  exportCsv: () =>
    downloadCsv("/categories/export", `categories-${new Date().toISOString().slice(0, 10)}.csv`),
};

// ---------- Inventory API ----------

export const inventoryApi = {
  list: (params: {
    page?: number;
    per_page?: number;
    product_id?: string;
    low_stock?: boolean;
    search?: string;
  } = {}) => requestList<InventoryItem>(`/inventory${buildQuery(params)}`),

  stockIn: (input: StockMovementRequest) =>
    request<StockMovementResponse>("/inventory/stock-in", {
      method: "POST",
      body: JSON.stringify(input),
    }),

  stockOut: (input: StockMovementRequest) =>
    request<StockMovementResponse>("/inventory/stock-out", {
      method: "POST",
      body: JSON.stringify(input),
    }),

  transactions: (params: {
    page?: number;
    per_page?: number;
    product_id?: string;
    type?: "IN" | "OUT";
  } = {}) => requestList<Transaction>(`/inventory/transactions${buildQuery(params)}`),

  exportCsv: () =>
    downloadCsv("/inventory/export", `inventory-${new Date().toISOString().slice(0, 10)}.csv`),
};

// ---------- User API (admin) ----------

export const userApi = {
  list: (params: {
    page?: number;
    per_page?: number;
    name?: string;
    email?: string;
    role?: "ADMIN" | "STAFF";
    is_active?: boolean;
  } = {}) => requestList<User>(`/users${buildQuery(params)}`),

  get: (id: string) => request<User>(`/users/${id}`),

  update: (id: string, input: UserInput) =>
    request<User>(`/users/${id}`, { method: "PUT", body: JSON.stringify(input) }),

  deactivate: (id: string) =>
    request<{ success: boolean }>(`/users/${id}`, { method: "DELETE" }),

  updateRole: (id: string, input: RoleUpdateRequest) =>
    request<User>(`/users/${id}/role`, { method: "PUT", body: JSON.stringify(input) }),
};

// ---------- Report API ----------

export const reportApi = {
  summary: () => request<StockSummary>("/reports/stock-summary"),

  exportCsv: () =>
    downloadCsv("/reports/export", `stock-summary-${new Date().toISOString().slice(0, 10)}.csv`),

  exportLowStockCsv: () =>
    downloadCsv("/reports/export-low-stock", `low-stock-${new Date().toISOString().slice(0, 10)}.csv`),
};

// ---------- Activity log API (admin) ----------

export const activityApi = {
  list: (params: {
    page?: number;
    per_page?: number;
    user_id?: string;
    entity_type?: string;
    entity_id?: string;
    action?: string;
    from?: string;
    to?: string;
  } = {}) => requestList<Activity>(`/activity-logs${buildQuery(params)}`),
};