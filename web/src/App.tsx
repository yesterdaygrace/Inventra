import { Navigate, Route, Routes } from "react-router-dom";
import { AppLayout } from "@/components/layout/app-layout";
import { RequireAuth, RequireGuest, RequireRole } from "@/lib/auth";
import { LoginPage } from "@/pages/login";
import { RegisterPage } from "@/pages/register";
import { DashboardPage } from "@/pages/dashboard";
import { ProductsPage } from "@/pages/products";
import { CategoriesPage } from "@/pages/categories";
import { InventoryPage } from "@/pages/inventory";
import { TransactionsPage } from "@/pages/transactions";
import { ReportsPage } from "@/pages/reports";
import { UsersPage } from "@/pages/users";
import { ActivityLogPage } from "@/pages/activity";
import { SettingsPage } from "@/pages/settings";

export function App() {
  return (
    <Routes>
      {/* Public auth */}
      <Route
        path="/login"
        element={
          <RequireGuest>
            <LoginPage />
          </RequireGuest>
        }
      />
      <Route
        path="/register"
        element={
          <RequireGuest>
            <RegisterPage />
          </RequireGuest>
        }
      />

      {/* Protected app shell */}
      <Route
        element={
          <RequireAuth>
            <AppLayout />
          </RequireAuth>
        }
      >
        <Route path="/" element={<DashboardPage />} />
        <Route path="/products" element={<ProductsPage />} />
        <Route path="/categories" element={<CategoriesPage />} />
        <Route path="/inventory" element={<InventoryPage />} />
        <Route path="/transactions" element={<TransactionsPage />} />
        <Route path="/reports" element={<ReportsPage />} />
        <Route
          path="/activity"
          element={
            <RequireRole roles={["ADMIN"]}>
              <ActivityLogPage />
            </RequireRole>
          }
        />
        <Route
          path="/users"
          element={
            <RequireRole roles={["ADMIN"]}>
              <UsersPage />
            </RequireRole>
          }
        />
        <Route path="/settings" element={<SettingsPage />} />
      </Route>

      {/* Fallback */}
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}