import * as React from "react";
import { Search, Download, Plus, Minus, ArrowLeftRight, MoreHorizontal } from "lucide-react";
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
import { StockMovementDialog, type MovementType } from "@/components/inventory/stock-movement-dialog";
import { TransferDialog } from "@/components/inventory/transfer-dialog";
import { inventoryApi, productApi, warehouseApi, isApiError } from "@/lib/api";
import { listKeys } from "@/lib/query";
import { useList, useApiMutation, useQueryClient } from "@/hooks/use-query";
import { useAuth } from "@/lib/auth";
import { formatNumber, formatDateTime } from "@/lib/format";
import type { InventoryItem, StockMovementRequest, TransferRequest } from "@/types/api";

const PER_PAGE = 10;

export function InventoryPage() {
  const { user } = useAuth();
  const isOperator = user?.role === "ADMIN" || user?.role === "STAFF";
  const queryClient = useQueryClient();
  const { toast } = useToast();

  const [page, setPage] = React.useState(1);
  const [search, setSearch] = React.useState("");
  const [debouncedSearch, setDebouncedSearch] = React.useState("");
  const [lowStock, setLowStock] = React.useState(false);
  const [dialogOpen, setDialogOpen] = React.useState(false);
  const [movementType, setMovementType] = React.useState<MovementType>("IN");
  const [movementProductId, setMovementProductId] = React.useState<string | undefined>(undefined);
  const [transferOpen, setTransferOpen] = React.useState(false);
  const [exporting, setExporting] = React.useState(false);

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
    low_stock: lowStock || undefined,
  };

  const inventory = useList(listKeys.inventory(listParams), () => inventoryApi.list(listParams));

  const products = useList(listKeys.products({ dropdown: true, is_archived: false }), () =>
    productApi.list({ per_page: 200, is_archived: false }),
  );

  const warehouses = useList(listKeys.warehouses({ dropdown: true }), () =>
    warehouseApi.list({ per_page: 200 }),
  );

  const handleExport = async () => {
    setExporting(true);
    try {
      await inventoryApi.exportCsv();
      toast({ title: "Inventory exported", variant: "success" });
    } catch (err) {
      toast({ title: isApiError(err) ? err.message : "Export failed", variant: "destructive" });
    } finally {
      setExporting(false);
    }
  };

  const invalidateAfterMovement = () => {
    queryClient.invalidateQueries({ queryKey: ["inventory"] });
    queryClient.invalidateQueries({ queryKey: ["products"] });
    queryClient.invalidateQueries({ queryKey: ["inventory", "transactions"] });
    queryClient.invalidateQueries({ queryKey: ["warehouses"] });
    queryClient.invalidateQueries({ queryKey: ["dashboard"] });
    queryClient.invalidateQueries({ queryKey: ["reports"] });
  };

  const movement = useApiMutation(
    (input: StockMovementRequest) =>
      movementType === "IN" ? inventoryApi.stockIn(input) : inventoryApi.stockOut(input),
    {
      onSuccess: () => {
        invalidateAfterMovement();
        toast({ title: `Stock ${movementType === "IN" ? "in" : "out"} complete`, variant: "success" });
      },
      onError: () => {},
    },
  );

  const transfer = useApiMutation((input: TransferRequest) => inventoryApi.transfer(input), {
    onSuccess: () => {
      invalidateAfterMovement();
      toast({ title: "Transfer complete", variant: "success" });
    },
    onError: () => {},
  });

  const openMovement = (type: MovementType, productId?: string) => {
    setMovementType(type);
    setMovementProductId(productId);
    setDialogOpen(true);
  };

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Inventory</h1>
          <p className="text-sm text-muted-foreground">
            Stock levels — {formatNumber(inventory.data?.pagination?.total)} items
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Button variant="outline" onClick={handleExport} disabled={exporting}>
            <Download className="h-4 w-4" />
            Export
          </Button>
          {isOperator && (
            <>
              <Button onClick={() => openMovement("IN")}>
                <Plus className="h-4 w-4" />
                Stock in
              </Button>
              <Button variant="secondary" onClick={() => openMovement("OUT")}>
                <Minus className="h-4 w-4" />
                Stock out
              </Button>
              <Button variant="outline" onClick={() => setTransferOpen(true)}>
                <ArrowLeftRight className="h-4 w-4" />
                Transfer
              </Button>
            </>
          )}
        </div>
      </div>

      <div className="flex flex-col gap-3 md:flex-row md:items-center">
        <div className="relative flex-1 md:max-w-xs">
          <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            className="pl-9"
            placeholder="Search name or SKU…"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            aria-label="Search inventory"
          />
        </div>
        <Button
          variant={lowStock ? "secondary" : "outline"}
          size="sm"
          onClick={() => {
            setLowStock((v) => !v);
            setPage(1);
          }}
        >
          Low stock
        </Button>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Stock levels</CardTitle>
        </CardHeader>
        <CardContent>
          {inventory.isLoading ? (
            <SkeletonList rows={6} />
          ) : inventory.isError ? (
            <ErrorState description="Could not load inventory." onRetry={() => inventory.refetch()} />
          ) : inventory.data?.items?.length ? (
            <InventoryTable
              items={inventory.data.items}
              isOperator={isOperator}
              onStockIn={(id) => openMovement("IN", id)}
              onStockOut={(id) => openMovement("OUT", id)}
            />
          ) : (
            <EmptyState
              title={search || lowStock ? "No matching items" : "No stock items yet"}
              description={
                search || lowStock ? "Try adjusting your filters." : "Stock appears here after products are created."
              }
            />
          )}

          {inventory.data?.pagination && inventory.data.items.length > 0 && (
            <Pagination
              className="mt-4"
              page={page}
              totalPages={inventory.data.pagination.total_pages}
              onPageChange={setPage}
            />
          )}
        </CardContent>
      </Card>

      <StockMovementDialog
        open={dialogOpen}
        onOpenChange={setDialogOpen}
        movementType={movementType}
        products={products.data?.items ?? []}
        warehouses={warehouses.data?.items ?? []}
        initialProductId={movementProductId}
        onSubmit={(values) => movement.mutateAsync(values).then(() => undefined)}
        submitting={movement.isPending}
      />

      <TransferDialog
        open={transferOpen}
        onOpenChange={setTransferOpen}
        products={products.data?.items ?? []}
        warehouses={warehouses.data?.items ?? []}
        onSubmit={(values) => transfer.mutateAsync(values).then(() => undefined)}
        submitting={transfer.isPending}
      />
    </div>
  );
}

function InventoryTable({
  items,
  isOperator,
  onStockIn,
  onStockOut,
}: {
  items: InventoryItem[];
  isOperator: boolean;
  onStockIn: (productId: string) => void;
  onStockOut: (productId: string) => void;
}) {
  return (
    <div className="overflow-x-auto">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Product</TableHead>
            <TableHead className="text-right">Quantity</TableHead>
            <TableHead>Status</TableHead>
            <TableHead>Updated</TableHead>
            {isOperator && <TableHead className="w-12" />}
          </TableRow>
        </TableHeader>
        <TableBody>
          {items.map((item) => (
            <TableRow key={item.product_id}>
              <TableCell>
                <p className="truncate font-medium">{item.product_name}</p>
                <p className="text-xs text-muted-foreground">{item.product_sku}</p>
              </TableCell>
              <TableCell className="text-right tabular-nums">{formatNumber(item.quantity)}</TableCell>
              <TableCell>
                {item.quantity === 0 ? (
                  <Badge variant="critical">Out of stock</Badge>
                ) : (
                  <Badge variant="healthy">In stock</Badge>
                )}
              </TableCell>
              <TableCell className="whitespace-nowrap text-muted-foreground">
                {formatDateTime(item.updated_at)}
              </TableCell>
              {isOperator && (
                <TableCell>
                  <DropdownMenu>
                    <DropdownMenuTrigger asChild>
                      <Button variant="ghost" size="icon" className="h-8 w-8" aria-label="Actions">
                        <MoreHorizontal className="h-4 w-4" />
                      </Button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="end">
                      <DropdownMenuItem onClick={() => onStockIn(item.product_id)}>
                        <Plus className="h-4 w-4" />
                        Stock in
                      </DropdownMenuItem>
                      <DropdownMenuItem onClick={() => onStockOut(item.product_id)}>
                        <Minus className="h-4 w-4" />
                        Stock out
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