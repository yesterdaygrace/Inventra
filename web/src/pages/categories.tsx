import * as React from "react";
import { Plus, MoreHorizontal, Download } from "lucide-react";
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
import { CategoryFormDialog } from "@/components/categories/category-form-dialog";
import { categoryApi, isApiError } from "@/lib/api";
import { listKeys } from "@/lib/query";
import { useList, useApiMutation, useQueryClient } from "@/hooks/use-query";
import { useAuth } from "@/lib/auth";
import { formatNumber, formatDateTime } from "@/lib/format";
import type { Category, CategoryInput } from "@/types/api";

const PER_PAGE = 10;

export function CategoriesPage() {
  const { user } = useAuth();
  const isAdmin = user?.role === "ADMIN";
  const queryClient = useQueryClient();
  const { toast } = useToast();

  const [page, setPage] = React.useState(1);
  const [name, setName] = React.useState("");
  const [debouncedName, setDebouncedName] = React.useState("");
  const [formOpen, setFormOpen] = React.useState(false);
  const [editing, setEditing] = React.useState<Category | null>(null);
  const [exporting, setExporting] = React.useState(false);

  React.useEffect(() => {
    const t = setTimeout(() => {
      setDebouncedName(name);
      setPage(1);
    }, 300);
    return () => clearTimeout(t);
  }, [name]);

  const listParams = {
    page,
    per_page: PER_PAGE,
    name: debouncedName || undefined,
  };

  const categories = useList(listKeys.categories(listParams), () => categoryApi.list(listParams));

  const saveCategory = useApiMutation(
    (input: CategoryInput) =>
      editing ? categoryApi.update(editing.id, input) : categoryApi.create(input),
    {
      onSuccess: () => {
        queryClient.invalidateQueries({ queryKey: ["categories"] });
        toast({
          title: editing ? "Category updated" : "Category created",
          variant: "success",
        });
      },
    },
  );

  const deactivateCategory = useApiMutation(
    (id: string) => categoryApi.deactivate(id),
    {
      onSuccess: () => {
        queryClient.invalidateQueries({ queryKey: ["categories"] });
        queryClient.invalidateQueries({ queryKey: ["products"] });
        toast({ title: "Category deactivated", variant: "success" });
      },
    },
  );

  const openCreate = () => {
    setEditing(null);
    setFormOpen(true);
  };

  const handleExport = async () => {
    setExporting(true);
    try {
      await categoryApi.exportCsv();
      toast({ title: "Categories exported", variant: "success" });
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
          <h1 className="text-2xl font-semibold tracking-tight">Categories</h1>
          <p className="text-sm text-muted-foreground">
            Organize your products — {formatNumber(categories.data?.pagination?.total)} categories
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
              Add category
            </Button>
          )}
        </div>
      </div>

      <div className="relative max-w-xs">
        <Input
          value={name}
          onChange={(e) => setName(e.target.value)}
          placeholder="Search categories…"
          aria-label="Search categories"
        />
      </div>

      <Card>
        <CardHeader>
          <CardTitle>All categories</CardTitle>
        </CardHeader>
        <CardContent>
          {categories.isLoading ? (
            <SkeletonList rows={6} />
          ) : categories.isError ? (
            <ErrorState description="Could not load categories." onRetry={() => categories.refetch()} />
          ) : categories.data?.items?.length ? (
            <CategoryTable
              items={categories.data.items}
              isAdmin={isAdmin}
              onEdit={(c) => {
                setEditing(c);
                setFormOpen(true);
              }}
              onToggleActive={(id) => deactivateCategory.mutate(id)}
            />
          ) : (            <EmptyState
              title={debouncedName ? "No matching categories" : "No categories yet"}
              description={
                debouncedName
                  ? "Try a different search term."
                  : "Create your first category to organize products."
              }
              action={isAdmin ? <Button onClick={openCreate}>Add category</Button> : undefined}
            />
          )}

          {categories.data?.pagination && categories.data.items.length > 0 && (
            <Pagination
              className="mt-4"
              page={page}
              totalPages={categories.data.pagination.total_pages}
              onPageChange={setPage}
            />
          )}
        </CardContent>
      </Card>

      <CategoryFormDialog
        open={formOpen}
        onOpenChange={setFormOpen}
        category={editing}
        onSubmit={(values) => saveCategory.mutateAsync(values).then(() => undefined)}
        submitting={saveCategory.isPending}
      />
    </div>
  );
}

function CategoryTable({
  items,
  isAdmin,
  onEdit,
  onToggleActive,
}: {
  items: Category[];
  isAdmin: boolean;
  onEdit: (c: Category) => void;
  onToggleActive: (id: string) => void;
}) {
  return (
    <div className="overflow-x-auto">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Name</TableHead>
            <TableHead>Description</TableHead>
            <TableHead className="text-right">Products</TableHead>
            <TableHead>Status</TableHead>
            <TableHead>Updated</TableHead>
            {isAdmin && <TableHead className="w-12" />}
          </TableRow>
        </TableHeader>
        <TableBody>
          {items.map((c) => (
            <TableRow key={c.id}>
              <TableCell className="font-medium">{c.name}</TableCell>
              <TableCell className="max-w-xs truncate text-muted-foreground">
                {c.description || "—"}
              </TableCell>
              <TableCell className="text-right tabular-nums">
                {c.product_count === undefined ? "—" : formatNumber(c.product_count)}
              </TableCell>
              <TableCell>
                <Badge variant={c.is_active === false ? "secondary" : "healthy"}>
                  {c.is_active === false ? "Inactive" : "Active"}
                </Badge>
              </TableCell>
              <TableCell className="whitespace-nowrap text-muted-foreground">
                {formatDateTime(c.updated_at)}
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
                      <DropdownMenuItem onClick={() => onEdit(c)}>Edit</DropdownMenuItem>
                      <DropdownMenuItem variant="destructive" onClick={() => onToggleActive(c.id)} disabled={c.is_active === false}>
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