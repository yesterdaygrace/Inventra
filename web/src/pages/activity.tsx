import * as React from "react";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
  Input,
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
import { activityApi } from "@/lib/api";
import { listKeys } from "@/lib/query";
import { useList } from "@/hooks/use-query";
import { formatNumber, formatDateTime } from "@/lib/format";

const PER_PAGE = 15;

const ENTITY_OPTIONS = [
  { value: "product", label: "Product" },
  { value: "category", label: "Category" },
  { value: "inventory", label: "Inventory" },
  { value: "user", label: "User" },
  { value: "auth", label: "Auth" },
  { value: "report", label: "Report" },
];

export function ActivityLogPage() {
  const [page, setPage] = React.useState(1);
  const [action, setAction] = React.useState("");
  const [debouncedAction, setDebouncedAction] = React.useState("");
  const [entityType, setEntityType] = React.useState("");

  React.useEffect(() => {
    const t = setTimeout(() => {
      setDebouncedAction(action);
      setPage(1);
    }, 300);
    return () => clearTimeout(t);
  }, [action]);

  const params = {
    page,
    per_page: PER_PAGE,
    action: debouncedAction || undefined,
    entity_type: entityType || undefined,
  };

  const logs = useList(listKeys.activity(params), () => activityApi.list(params));

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">Activity Log</h1>
        <p className="text-sm text-muted-foreground">
          Audit trail — {formatNumber(logs.data?.pagination?.total)} events
        </p>
      </div>

      <div className="flex flex-wrap items-center gap-2">
        <Input
          className="w-44"
          placeholder="Filter by action…"
          value={action}
          onChange={(e) => setAction(e.target.value)}
          aria-label="Filter by action"
        />
        <Select
          className="w-44"
          aria-label="Filter by entity"
          placeholder="All entities"
          value={entityType}
          onChange={(e) => {
            setEntityType(e.target.value);
            setPage(1);
          }}
          options={ENTITY_OPTIONS}
        />
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Events</CardTitle>
        </CardHeader>
        <CardContent>
          {logs.isLoading ? (
            <SkeletonList rows={8} />
          ) : logs.isError ? (
            <ErrorState description="Could not load the activity log." onRetry={() => logs.refetch()} />
          ) : logs.data?.items?.length ? (
            <div className="overflow-x-auto">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Time</TableHead>
                    <TableHead>User</TableHead>
                    <TableHead>Action</TableHead>
                    <TableHead>Entity</TableHead>
                    <TableHead>Details</TableHead>
                    <TableHead>IP</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {logs.data.items.map((a) => (
                    <TableRow key={a.id}>
                      <TableCell className="whitespace-nowrap text-muted-foreground">
                        {formatDateTime(a.created_at)}
                      </TableCell>
                      <TableCell className="font-medium">{a.user_name || "—"}</TableCell>
                      <TableCell>{a.action}</TableCell>
                      <TableCell className="text-muted-foreground">{a.entity_type}</TableCell>
                      <TableCell className="max-w-[16rem]">
                        <pre className="truncate text-xs text-muted-foreground">
                          {formatDetails(a.details)}
                        </pre>
                      </TableCell>
                      <TableCell className="text-muted-foreground">{a.ip || "—"}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          ) : (
            <EmptyState title="No activity" description="No events match your filters." />
          )}

          {logs.data?.pagination && logs.data.items.length > 0 && (
            <Pagination
              className="mt-4"
              page={page}
              totalPages={logs.data.pagination.total_pages}
              onPageChange={setPage}
            />
          )}
        </CardContent>
      </Card>
    </div>
  );
}

function formatDetails(details: unknown): string {
  if (details == null) return "—";
  if (typeof details === "string") return details;
  try {
    return JSON.stringify(details);
  } catch {
    return "—";
  }
}