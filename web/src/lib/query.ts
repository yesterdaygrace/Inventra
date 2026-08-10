import { QueryClient } from "@tanstack/react-query";
import { ApiError } from "@/lib/api";
import { toast } from "@/components/ui";

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000,
      gcTime: 5 * 60_000,
      retry: (failureCount, error) => {
        if (error instanceof ApiError && error.status >= 400 && error.status < 500) {
          return false;
        }
        return failureCount < 2;
      },
      refetchOnWindowFocus: true,
    },
    mutations: {
      retry: false,
      onError: (error) => {
        if (error instanceof ApiError) {
          toast({
            variant: "destructive",
            title: "Request failed",
            description: error.message,
          });
        }
      },
    },
  },
});

export const listKeys = {
  products: (params: unknown) => ["products", params] as const,
  categories: (params: unknown) => ["categories", params] as const,
  warehouses: (params: unknown) => ["warehouses", params] as const,
  inventory: (params: unknown) => ["inventory", params] as const,
  transactions: (params: unknown) => ["transactions", params] as const,
  users: (params: unknown) => ["users", params] as const,
  activity: (params: unknown) => ["activity", params] as const,
  reports: () => ["reports", "stock-summary"] as const,
  dashboardSummary: (p: {}) => ["dashboard", "summary", p] as const,
  dashboardMovement: () => ["dashboard", "movement"] as const,
  dashboardCategory: () => ["dashboard", "category"] as const,
  dashboardTopSelling: () => ["dashboard", "top-selling"] as const,
};