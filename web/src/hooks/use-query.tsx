import {
  useQuery,
  useMutation,
  useQueryClient,
  type QueryKey,
  type UseQueryOptions,
  type UseMutationOptions,
  type UseMutationResult,
} from "@tanstack/react-query";
import type { ListResult } from "@/types/api";

export function useList<T>(
  key: QueryKey,
  fn: () => Promise<ListResult<T>>,
  options?: Omit<
    UseQueryOptions<ListResult<T>, unknown, ListResult<T>, QueryKey>,
    "queryKey" | "queryFn"
  >,
) {
  return useQuery<ListResult<T>, unknown, ListResult<T>, QueryKey>({
    queryKey: key,
    queryFn: fn,
    ...options,
  });
}

export function useApiQuery<T>(
  key: QueryKey,
  fn: () => Promise<T>,
  options?: Omit<UseQueryOptions<T, unknown, T, QueryKey>, "queryKey" | "queryFn">,
) {
  return useQuery<T, unknown, T, QueryKey>({
    queryKey: key,
    queryFn: fn,
    ...options,
  });
}

export function useApiMutation<TData, TVariables>(
  fn: (vars: TVariables) => Promise<TData>,
  options?: Omit<UseMutationOptions<TData, unknown, TVariables, unknown>, "mutationFn">,
): UseMutationResult<TData, unknown, TVariables, unknown> {
  return useMutation<TData, unknown, TVariables, unknown>({
    mutationFn: fn,
    ...options,
  });
}

export { useQueryClient };
export type { QueryKey };