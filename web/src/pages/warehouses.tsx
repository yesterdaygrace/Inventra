import * as React from "react";
import { Plus, MoreHorizontal } from "lucide-react";
import {
  Badge,
  Button,
  Card,
  CardContent,
  CardHeader,
  CardTitle,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
  Input,
  Pagination,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
  useToast,
} from "@/components/ui";
import { EmptyState, ErrorState, SkeletonList } from "@/components/ui/states";
import { WarehouseFormDialog } from "@/components/warehouses/warehouse-form-dialog";
import { warehouseApi } from "@/lib/api";
import { listKeys } from "@/lib/query";
import { useList, useApiMutation, useQueryClient } from "@/hooks/use-query";
import { useAuth } from "@/lib/auth";
import { formatNumber, formatDateTime } from "@/lib/format";
import type { Warehouse, WarehouseInput } from "@/types/api";

const PER_PAGE = 10;

export function WarehousesPage() {
  const { user } = useAuth();
  const isAdmin = user?.role === "ADMIN";
  const queryClient = useQueryClient();
  const { toast } = useToast();

  const [page, setPage] = React.useState(1);
  const [search, setSearch] = React.useState("");
  const [debouncedSearch, setDebouncedSearch] = React.useState("");
  const [formOpen, setFormOpen] = React.useState(false);
  const [editing, setEditing] = React.useState<Warehouse | null>(null);

  React.useEffect(() => {
    const t = setTimeout(() => {
      setDebouncedSearch(search);
      setPage(1);
    }, 300);
    return () => clearTimeout(t);
  }, [search]);

  const listParams = {
    page,
    per_page: PER_PAGE,
    search: debouncedSearch || undefined,
  };

  const warehouses = useList(listKeys.warehouses(listParams), () => warehouseApi.list(listParams));

  const saveWarehouse = useApiMutation(
    (input: WarehouseInput) =>
      editing ? warehouseApi.update(editing.id, input) : warehouseApi.create(input),
    {
      onSuccess: () => {
        queryClient.invalidateQueries({ queryKey: ["warehouses"] });
        toast({
          title: editing ? "Warehouse updated" : "Warehouse created",
          variant: "success",
        });
      },
    },
  );

  const deactivateWarehouse = useApiMutation(
    (id: string) => warehouseApi.deactivate(id),
    {
      onSuccess: () => {
        queryClient.invalidateQueries({ queryKey: ["warehouses"] });
        toast({ title: "Warehouse deactivated", variant: "success" });
      },
    },
  );

  const openCreate = () => {
    setEditing(null);
    setFormOpen(true);
  };

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Warehouses</h1>
          <p className="text-sm text-muted-foreground">
            Stock locations — {formatNumber(warehouses.data?.pagination?.total)} warehouses
          </p>
        </div>
        {isAdmin && (
          <Button onClick={openCreate}>
            <Plus className="h-4 w-4" />
            Add warehouse
          </Button>
        )}
      </div>

      <div className="relative max-w-xs">
        <Input
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder="Search name or code…"
          aria-label="Search warehouses"
        />
      </div>

      <Card>
        <CardHeader>
          <CardTitle>All warehouses</CardTitle>
        </CardHeader>
        <CardContent>
          {warehouses.isLoading ? (
            <SkeletonList rows={6} />
          ) : warehouses.isError ? (
            <ErrorState description="Could not load warehouses." onRetry={() => warehouses.refetch()} />
          ) : warehouses.data?.items?.length ? (
            <WarehouseTable
              items={warehouses.data.items}
              isAdmin={isAdmin}
              onEdit={(w) => {
                setEditing(w);
                setFormOpen(true);
              }}
              onToggleActive={(id) => deactivateWarehouse.mutate(id)}
            />
          ) : (
            <EmptyState
              title={debouncedSearch ? "No matching warehouses" : "No warehouses yet"}
              description={
                debouncedSearch
                  ? "Try a different search term."
                  : "Create your first warehouse to track stock by location."
              }
              action={isAdmin ? <Button onClick={openCreate}>Add warehouse</Button> : undefined}
            />
          )}

          {warehouses.data?.pagination && warehouses.data.items.length > 0 && (
            <Pagination
              className="mt-4"
              page={page}
              totalPages={warehouses.data.pagination.total_pages}
              onPageChange={setPage}
            />
          )}
        </CardContent>
      </Card>

      <WarehouseFormDialog
        open={formOpen}
        onOpenChange={setFormOpen}
        warehouse={editing}
        onSubmit={(values) => saveWarehouse.mutateAsync(values).then(() => undefined)}
        submitting={saveWarehouse.isPending}
      />
    </div>
  );
}

function WarehouseTable({
  items,
  isAdmin,
  onEdit,
  onToggleActive,
}: {
  items: Warehouse[];
  isAdmin: boolean;
  onEdit: (w: Warehouse) => void;
  onToggleActive: (id: string) => void;
}) {
  return (
    <div className="overflow-x-auto">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Code</TableHead>
            <TableHead>Name</TableHead>
            <TableHead>Description</TableHead>
            <TableHead className="text-right">Inventory</TableHead>
            <TableHead>Status</TableHead>
            <TableHead>Updated</TableHead>
            {isAdmin && <TableHead className="w-12" />}
          </TableRow>
        </TableHeader>
        <TableBody>
          {items.map((w) => (
            <TableRow key={w.id}>
              <TableCell className="font-mono text-xs font-medium">{w.code}</TableCell>
              <TableCell className="font-medium">{w.name}</TableCell>
              <TableCell className="max-w-xs truncate text-muted-foreground">
                {w.description || "—"}
              </TableCell>
              <TableCell className="text-right tabular-nums">
                {w.inventory_count === undefined ? "—" : formatNumber(w.inventory_count)}
              </TableCell>
              <TableCell>
                <Badge variant={w.is_active === false ? "secondary" : "healthy"}>
                  {w.is_active === false ? "Inactive" : "Active"}
                </Badge>
              </TableCell>
              <TableCell className="whitespace-nowrap text-muted-foreground">
                {formatDateTime(w.updated_at)}
              </TableCell>
              {isAdmin && (
                <TableCell>
                  <DropdownMenu>
                    <DropdownMenuTrigger asChild>
                      <Button variant="ghost" size="icon" className="h-8 w-8" aria-label="Actions">
                        <MoreHorizontal className="h-4 w-4" />
                      </Button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="end">
                      <DropdownMenuItem onClick={() => onEdit(w)}>Edit</DropdownMenuItem>
                      <DropdownMenuItem
                        variant="destructive"
                        onClick={() => onToggleActive(w.id)}
                        disabled={w.is_active === false}
                      >
                        Deactivate
                      </DropdownMenuItem>
                    </DropdownMenuContent>
                  </DropdownMenu>
                </TableCell>
              )}
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}