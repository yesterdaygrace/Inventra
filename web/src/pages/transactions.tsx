import * as React from "react";
import {
  Badge,
  Card,
  CardContent,
  CardHeader,
  CardTitle,
  Pagination,
  Select,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui";
import { EmptyState, ErrorState, SkeletonList } from "@/components/ui/states";
import { inventoryApi, productApi } from "@/lib/api";
import { listKeys } from "@/lib/query";
import { useList } from "@/hooks/use-query";
import { formatCurrency, formatNumber, formatDateTime } from "@/lib/format";
import type { Transaction } from "@/types/api";

const PER_PAGE = 10;

const TYPE_OPTIONS = [
  { value: "IN", label: "Stock in" },
  { value: "OUT", label: "Stock out" },
];

export function TransactionsPage() {
  const [page, setPage] = React.useState(1);
  const [productId, setProductId] = React.useState("");
  const [type, setType] = React.useState<"" | "IN" | "OUT">("");

  const params = {
    page,
    per_page: PER_PAGE,
    product_id: productId || undefined,
    type: type || undefined,
  };

  const transactions = useList(listKeys.transactions(params), () =>
    inventoryApi.transactions(params),
  );

  const products = useList(listKeys.products({ dropdown: true, is_archived: false }), () =>
    productApi.list({ per_page: 200, is_archived: false }),
  );

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">Transactions</h1>
        <p className="text-sm text-muted-foreground">
          Movement history — {formatNumber(transactions.data?.pagination?.total)} records
        </p>
      </div>

      <div className="flex flex-wrap items-center gap-2">
        <Select
          className="w-56"
          aria-label="Filter by product"
          placeholder="All products"
          value={productId}
          onChange={(e) => {
            setProductId(e.target.value);
            setPage(1);
          }}
          options={(products.data?.items ?? []).map((p) => ({ value: p.id, label: p.name }))}
        />
        <Select
          className="w-40"
          aria-label="Filter by type"
          placeholder="All types"
          value={type}
          onChange={(e) => {
            setType(e.target.value as "" | "IN" | "OUT");
            setPage(1);
          }}
          options={TYPE_OPTIONS}
        />
      </div>

      <Card>
        <CardHeader>
          <CardTitle>History</CardTitle>
        </CardHeader>
        <CardContent>
          {transactions.isLoading ? (
            <SkeletonList rows={8} />
          ) : transactions.isError ? (
            <ErrorState description="Could not load transactions." onRetry={() => transactions.refetch()} />
          ) : transactions.data?.items?.length ? (
            <div className="overflow-x-auto">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Item</TableHead>
                    <TableHead>Date</TableHead>
                    <TableHead>Type</TableHead>
                    <TableHead className="text-right">Qty</TableHead>
                    <TableHead className="text-right">Unit cost</TableHead>
                    <TableHead>Note</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {transactions.data.items.map((tx) => (
                    <TransactionRow key={tx.id} tx={tx} />
                  ))}
                </TableBody>
              </Table>
            </div>
          ) : (
            <EmptyState
              title="No transactions yet"
              description="Stock movements appear here as you stock items in or out."
            />
          )}

          {transactions.data?.pagination && transactions.data.items.length > 0 && (
            <Pagination
              className="mt-4"
              page={page}
              totalPages={transactions.data.pagination.total_pages}
              onPageChange={setPage}
            />
          )}
        </CardContent>
      </Card>
    </div>
  );
}

function TransactionRow({ tx }: { tx: Transaction }) {
  return (
    <TableRow>
      <TableCell>
        <p className="truncate font-medium">{tx.product_name}</p>
        <p className="text-xs text-muted-foreground">{tx.product_sku}</p>
      </TableCell>
      <TableCell className="whitespace-nowrap text-muted-foreground">
        {formatDateTime(tx.created_at)}
      </TableCell>
      <TableCell>
        {tx.type === "IN" ? (
          <Badge variant="healthy">In</Badge>
        ) : (
          <Badge variant="secondary">Out</Badge>
        )}
      </TableCell>
      <TableCell className="text-right tabular-nums">{formatNumber(tx.quantity)}</TableCell>
      <TableCell className="text-right tabular-nums">
        {tx.unit_cost === undefined ? "—" : formatCurrency(tx.unit_cost)}
      </TableCell>
      <TableCell className="max-w-[12rem] truncate text-muted-foreground">
        {tx.note || "—"}
      </TableCell>
    </TableRow>
  );
}