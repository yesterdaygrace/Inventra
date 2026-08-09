import * as React from "react";
import { Search, Plus, MoreHorizontal, FilterX, Download } from "lucide-react";
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
  Select,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
  useToast,
} from "@/components/ui";
import { EmptyState, ErrorState, SkeletonList } from "@/components/ui/states";
import { ProductFormDialog } from "@/components/products/product-form-dialog";
import { productApi, categoryApi, isApiError } from "@/lib/api";
import { listKeys } from "@/lib/query";
import { useList, useApiMutation, useQueryClient } from "@/hooks/use-query";
import { useAuth } from "@/lib/auth";
import { formatCurrency, formatNumber, formatDateTime } from "@/lib/format";
import type { Product, ProductInput } from "@/types/api";

const PER_PAGE = 10;

export function ProductsPage() {
  const { user } = useAuth();
  const isAdmin = user?.role === "ADMIN";
  const queryClient = useQueryClient();
  const { toast } = useToast();

  const [page, setPage] = React.useState(1);
  const [search, setSearch] = React.useState("");
  const [debouncedSearch, setDebouncedSearch] = React.useState("");
  const [categoryId, setCategoryId] = React.useState("");
  const [sort, setSort] = React.useState("");
  const [showArchived, setShowArchived] = React.useState(false);
  const [lowStock, setLowStock] = React.useState(false);
  const [formOpen, setFormOpen] = React.useState(false);
  const [editing, setEditing] = React.useState<Product | null>(null);
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
    q: debouncedSearch || undefined,
    category_id: categoryId || undefined,
    is_archived: showArchived ? undefined : false,
    low_stock: lowStock || undefined,
    sort: sort || undefined,
  };

  const products = useList(listKeys.products(listParams), () => productApi.list(listParams));

  const categories = useList(listKeys.categories({ per_page: 100 }), () =>
    categoryApi.list({ per_page: 100 }),
  );

  const saveProduct = useApiMutation(
    (input: ProductInput) =>
      editing ? productApi.update(editing.id, input) : productApi.create(input),
    {
      onSuccess: () => {
        queryClient.invalidateQueries({ queryKey: ["products"] });
        toast({
          title: editing ? "Product updated" : "Product created",
          variant: "success",
        });
      },
    },
  );

  const toggleArchive = useApiMutation(
    (id: string) => productApi.archive(id),
    {
      onSuccess: () => {
        queryClient.invalidateQueries({ queryKey: ["products"] });
        toast({ title: "Product archived", variant: "success" });
      },
    },
  );

  const hasActiveFilters = !!search || !!categoryId || lowStock;

  const openCreate = () => {
    setEditing(null);
    setFormOpen(true);
  };

  const handleExport = async () => {
    setExporting(true);
    try {
      await productApi.exportCsv();
      toast({ title: "Products exported", variant: "success" });
    } catch (err) {
      toast({
        title: isApiError(err) ? err.message : "Export failed",
        variant: "destructive",
      });
    } finally {
      setExporting(false);
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Products</h1>
          <p className="text-sm text-muted-foreground">
            Manage your catalog — {formatNumber(products.data?.pagination?.total)} products
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Button variant="outline" onClick={handleExport} disabled={exporting}>
            <Download className="h-4 w-4" />
            Export
          </Button>
          {isAdmin && (
            <Button onClick={openCreate}>
              <Plus className="h-4 w-4" />
              Add product
            </Button>
          )}
        </div>
      </div>

      {/* Filters */}
      <div className="flex flex-col gap-3 md:flex-row md:items-center">
        <div className="relative flex-1 md:max-w-xs">
          <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            className="pl-9"
            placeholder="Search name or SKU…"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            aria-label="Search products"
          />
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <Select
            className="w-44"
            aria-label="Filter by category"
            placeholder="All categories"
            value={categoryId}
            onChange={(e) => {
              setCategoryId(e.target.value);
              setPage(1);
            }}
            options={(categories.data?.items ?? []).map((c) => ({ value: c.id, label: c.name }))}
          />
          <Select
            className="w-40"
            aria-label="Sort products"
            placeholder="Sort by"
            value={sort}
            onChange={(e) => {
              setSort(e.target.value);
              setPage(1);
            }}
            options={[
              { value: "-name", label: "Name (Z–A)" },
              { value: "price", label: "Price: low to high" },
              { value: "-price", label: "Price: high to low" },
              { value: "-created_at", label: "Newest" },
              { value: "created_at", label: "Oldest" },
            ]}
          />
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
          <Button
            variant={showArchived ? "secondary" : "outline"}
            size="sm"
            onClick={() => {
              setShowArchived((v) => !v);
              setPage(1);
            }}
          >
            Archived
          </Button>
          {hasActiveFilters && (
            <Button
              variant="ghost"
              size="sm"
              aria-label="Clear filters"
              onClick={() => {
                setCategoryId("");
                setLowStock(false);
                setShowArchived(false);
                setSearch("");
                setDebouncedSearch("");
                setPage(1);
              }}
            >
              <FilterX className="h-4 w-4" />
              Clear
            </Button>
          )}
        </div>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Catalog</CardTitle>
        </CardHeader>
        <CardContent>
          {products.isLoading ? (
            <SkeletonList rows={6} />
          ) : products.isError ? (
            <ErrorState description="Could not load products." onRetry={() => products.refetch()} />
          ) : products.data?.items?.length ? (
            <ProductTable
              items={products.data.items}
              isAdmin={isAdmin}
              onEdit={(p) => {
                setEditing(p);
                setFormOpen(true);
              }}
              onToggleArchive={(id) => toggleArchive.mutate(id)}
            />
          ) : (
            <EmptyState
              title={hasActiveFilters ? "No matching products" : "No products yet"}
              description={
                hasActiveFilters
                  ? "Try adjusting your filters."
                  : "Add your first product to get started."
              }
              action={isAdmin ? <Button onClick={openCreate}>Add product</Button> : undefined}
            />
          )}

          {products.data?.pagination && products.data.items.length > 0 && (
            <Pagination
              className="mt-4"
              page={page}
              totalPages={products.data.pagination.total_pages}
              onPageChange={setPage}
            />
          )}
        </CardContent>
      </Card>

      <ProductFormDialog
        open={formOpen}
        onOpenChange={setFormOpen}
        product={editing}
        categories={categories.data?.items ?? []}
        onSubmit={(values) => saveProduct.mutateAsync(values).then(() => undefined)}
        submitting={saveProduct.isPending}
      />
    </div>
  );
}

function ProductTable({
  items,
  isAdmin,
  onEdit,
  onToggleArchive,
}: {
  items: Product[];
  isAdmin: boolean;
  onEdit: (p: Product) => void;
  onToggleArchive: (id: string) => void;
}) {
  return (
    <div className="overflow-x-auto">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Product</TableHead>
            <TableHead>Category</TableHead>
            <TableHead className="text-right">Price</TableHead>
            <TableHead className="text-right">Stock</TableHead>
            <TableHead>Status</TableHead>
            <TableHead>Updated</TableHead>
            {isAdmin && <TableHead className="w-12" />}
          </TableRow>
        </TableHeader>
        <TableBody>
          {items.map((p) => (
            <TableRow key={p.id}>
              <TableCell>
                <p className="truncate font-medium">{p.name}</p>
                <p className="text-xs text-muted-foreground">{p.sku}</p>
              </TableCell>
              <TableCell className="text-muted-foreground">{p.category_name ?? "—"}</TableCell>
              <TableCell className="text-right tabular-nums">{formatCurrency(p.price)}</TableCell>
              <TableCell className="text-right tabular-nums">
                {p.stock_quantity === undefined ? "—" : formatNumber(p.stock_quantity)}
              </TableCell>
              <TableCell>
                <ProductStatusBadge p={p} />
              </TableCell>
              <TableCell className="whitespace-nowrap text-muted-foreground">
                {formatDateTime(p.updated_at)}
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
                      <DropdownMenuItem onClick={() => onEdit(p)}>Edit</DropdownMenuItem>
                      <DropdownMenuItem variant="destructive" onClick={() => onToggleArchive(p.id)}>
                        Archive
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

function ProductStatusBadge({ p }: { p: Product }) {
  if (p.is_archived) return <Badge variant="secondary">Archived</Badge>;
  if (
    p.is_low_stock ||
    (p.stock_quantity !== undefined && p.stock_quantity <= p.low_stock_threshold)
  ) {
    return <Badge variant="warning">Low stock</Badge>;
  }
  return <Badge variant="healthy">In stock</Badge>;
}