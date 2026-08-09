import * as React from "react";
import { Download } from "lucide-react";
import {
  Button,
  Card,
  CardContent,
  CardHeader,
  CardTitle,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
  useToast,
} from "@/components/ui";
import { EmptyState, ErrorState } from "@/components/ui/states";
import { reportApi, isApiError } from "@/lib/api";
import { useApiQuery } from "@/hooks/use-query";
import { listKeys } from "@/lib/query";
import { formatCurrency, formatNumber } from "@/lib/format";

export function ReportsPage() {
  const { toast } = useToast();
  const [exporting, setExporting] = React.useState(false);
  const [exportingLow, setExportingLow] = React.useState(false);

  const summary = useApiQuery(listKeys.reports(), () => reportApi.summary());

  const handleExport = async (kind: "summary" | "low") => {
    const setBusy = kind === "summary" ? setExporting : setExportingLow;
    setBusy(true);
    try {
      if (kind === "summary") await reportApi.exportCsv();
      else await reportApi.exportLowStockCsv();
      toast({ title: "Report exported", variant: "success" });
    } catch (err) {
      toast({ title: isApiError(err) ? err.message : "Export failed", variant: "destructive" });
    } finally {
      setBusy(false);
    }
  };

  const data = summary.data;

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Reports</h1>
          <p className="text-sm text-muted-foreground">Stock summary across your catalog</p>
        </div>
        <div className="flex items-center gap-2">
          <Button variant="outline" onClick={() => handleExport("summary")} disabled={exporting || !data}>
            <Download className="h-4 w-4" />
            Export summary
          </Button>
          <Button
            variant="outline"
            onClick={() => handleExport("low")}
            disabled={exportingLow || !data}
          >
            <Download className="h-4 w-4" />
            Export low stock
          </Button>
        </div>
      </div>

      {summary.isLoading ? (
        <SkeletonReport />
      ) : summary.isError ? (
        <ErrorState description="Could not load reports." onRetry={() => summary.refetch()} />
      ) : data ? (
        <>
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-2">
            <Card>
              <CardHeader>
                <CardTitle className="text-sm font-medium text-muted-foreground">
                  Total products
                </CardTitle>
              </CardHeader>
              <CardContent>
                <p className="text-3xl font-semibold tracking-tight">
                  {formatNumber(data.total_products)}
                </p>
              </CardContent>
            </Card>
            <Card>
              <CardHeader>
                <CardTitle className="text-sm font-medium text-muted-foreground">
                  Total stock value
                </CardTitle>
              </CardHeader>
              <CardContent>
                <p className="text-3xl font-semibold tracking-tight">
                  {formatCurrency(data.total_value)}
                </p>
              </CardContent>
            </Card>
          </div>

          <Card>
            <CardHeader>
              <CardTitle>Stock by category</CardTitle>
            </CardHeader>
            <CardContent>
              {data.categories.length ? (
                <div className="overflow-x-auto">
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>Category</TableHead>
                        <TableHead className="text-right">Products</TableHead>
                        <TableHead className="text-right">Quantity</TableHead>
                        <TableHead className="text-right">Value</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {data.categories.map((cat) => (
                        <TableRow key={cat.name}>
                          <TableCell className="font-medium">{cat.name}</TableCell>
                          <TableCell className="text-right tabular-nums">
                            {formatNumber(cat.product_count)}
                          </TableCell>
                          <TableCell className="text-right tabular-nums">
                            {formatNumber(cat.total_qty)}
                          </TableCell>
                          <TableCell className="text-right tabular-nums">
                            {formatCurrency(cat.total_value)}
                          </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                </div>
              ) : (
                <EmptyState title="No categories" description="No category stock to report yet." />
              )}
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>Low stock items</CardTitle>
            </CardHeader>
            <CardContent>
              {data.low_stock.length ? (
                <div className="overflow-x-auto">
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>Product</TableHead>
                        <TableHead>Category</TableHead>
                        <TableHead className="text-right">Quantity</TableHead>
                        <TableHead className="text-right">Threshold</TableHead>
                        <TableHead className="text-right">Value</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {data.low_stock.map((item) => (
                        <TableRow key={item.product_id}>
                          <TableCell>
                            <p className="truncate font-medium">{item.name}</p>
                            <p className="text-xs text-muted-foreground">{item.sku}</p>
                          </TableCell>
                          <TableCell className="text-muted-foreground">{item.category}</TableCell>
                          <TableCell className="text-right tabular-nums">
                            {formatNumber(item.quantity)}
                          </TableCell>
                          <TableCell className="text-right tabular-nums">
                            {formatNumber(item.threshold)}
                          </TableCell>
                          <TableCell className="text-right tabular-nums">
                            {formatCurrency(item.value)}
                          </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                </div>
              ) : (
                <EmptyState
                  title="No low-stock items"
                  description="All products are above their thresholds."
                />
              )}
            </CardContent>
          </Card>
        </>
      ) : null}
    </div>
  );
}

function SkeletonReport() {
  return (
    <div className="space-y-4">
      <div className="grid gap-4 sm:grid-cols-2">
        {[0, 1].map((i) => (
          <div key={i} className="h-24 animate-pulse rounded-xl bg-muted" />
        ))}
      </div>
      <div className="h-64 animate-pulse rounded-xl bg-muted" />
      <div className="h-48 animate-pulse rounded-xl bg-muted" />
    </div>
  );
}