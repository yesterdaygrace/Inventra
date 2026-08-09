import * as React from "react";
import { Link } from "react-router-dom";
import {
  Boxes,
  Package,
  DollarSign,
  AlertTriangle,
  ArrowDownToLine,
  ArrowUpFromLine,
  Clock,
  PackagePlus,
  ClipboardList,
} from "lucide-react";
import {
  Area,
  AreaChart,
  Bar,
  BarChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import {
  Badge,
  Button,
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui";
import { EmptyState, ErrorState, SkeletonList } from "@/components/ui/states";
import { dashboardApi } from "@/lib/api";
import { listKeys } from "@/lib/query";
import { useApiQuery } from "@/hooks/use-query";
import { useAuth } from "@/lib/auth";
import { formatCurrency, formatNumber, timeAgo } from "@/lib/format";
import { cn } from "@/lib/utils";

export function DashboardPage() {
  const { user } = useAuth();
  const isAdmin = user?.role === "ADMIN";

  const summary = useApiQuery(listKeys.dashboardSummary({}), () => dashboardApi.summary());
  const movement = useApiQuery(listKeys.dashboardMovement(), () => dashboardApi.movement());
  const category = useApiQuery(listKeys.dashboardCategory(), () => dashboardApi.categoryDistribution());
  const topSelling = useApiQuery(listKeys.dashboardTopSelling(), () => dashboardApi.topSelling());

  const summaryData = summary.data;

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Dashboard</h1>
          <p className="text-sm text-muted-foreground">
            Welcome back, {user?.name.split(" ")[0] ?? "there"}. Here's what's happening in your
            warehouse.
          </p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" size="sm" asChild>
            <Link to="/inventory">
              <ArrowDownToLine className="h-4 w-4" />
              Stock In
            </Link>
          </Button>
          <Button variant="outline" size="sm" asChild>
            <Link to="/inventory">
              <ArrowUpFromLine className="h-4 w-4" />
              Stock Out
            </Link>
          </Button>
        </div>
      </div>

      {/* KPI cards */}
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <KpiCard
          label="Total Products"
          value={summary.isLoading ? undefined : formatNumber(summaryData?.total_products)}
          icon={Package}
          tone="default"
        />
        <KpiCard
          label="Total Categories"
          value={summary.isLoading ? undefined : formatNumber(summaryData?.total_categories)}
          icon={Boxes}
          tone="default"
        />
        <KpiCard
          label="Inventory Value"
          value={summary.isLoading ? undefined : formatCurrency(summaryData?.inventory_value)}
          icon={DollarSign}
          tone="default"
        />
        <KpiCard
          label="Low Stock Items"
          value={summary.isLoading ? undefined : formatNumber(summaryData?.low_stock_count)}
          icon={AlertTriangle}
          tone={summaryData && summaryData.low_stock_count > 0 ? "warning" : "default"}
        />
      </div>

      {/* Charts row */}
      <div className="grid grid-cols-1 gap-4 xl:grid-cols-3">
        <Card className="xl:col-span-2">
          <CardHeader>
            <CardTitle>Inventory Movement</CardTitle>
            <CardDescription>Stock in, out, and net over the last 30 days</CardDescription>
          </CardHeader>
          <CardContent>
            {movement.isLoading ? (
              <SkeletonList rows={4} />
            ) : movement.isError ? (
              <ErrorState description="Could not load inventory movement." onRetry={() => movement.refetch()} />
            ) : movement.data?.datasets?.some((d) => d.data.some((v) => v > 0)) ? (
              <div className="h-72">
                <ResponsiveContainer width="100%" height="100%">
                  <AreaChart
                    data={movement.data.labels.map((label, i) => ({
                      name: label,
                      ...Object.fromEntries(movement.data!.datasets.map((d) => [d.label, d.data[i] ?? 0])),
                    }))}
                    margin={{ top: 8, right: 8, bottom: 0, left: -12 }}
                  >
                    <defs>
                      <linearGradient id="gIn" x1="0" y1="0" x2="0" y2="1">
                        <stop offset="0%" stopColor="var(--color-health)" stopOpacity={0.35} />
                        <stop offset="100%" stopColor="var(--color-health)" stopOpacity={0} />
                      </linearGradient>
                      <linearGradient id="gOut" x1="0" y1="0" x2="0" y2="1">
                        <stop offset="0%" stopColor="var(--color-critical)" stopOpacity={0.35} />
                        <stop offset="100%" stopColor="var(--color-critical)" stopOpacity={0} />
                      </linearGradient>
                    </defs>
                    <CartesianGrid strokeDasharray="3 3" stroke="var(--color-border)" vertical={false} />
                    <XAxis dataKey="name" tick={{ fontSize: 11, fill: "var(--color-muted-foreground)" }} tickLine={false} axisLine={false} interval="preserveStartEnd" minTickGap={32} />
                    <YAxis tick={{ fontSize: 11, fill: "var(--color-muted-foreground)" }} tickLine={false} axisLine={false} width={48} />
                    <Tooltip
                      contentStyle={{
                        backgroundColor: "var(--color-card)",
                        border: "1px solid var(--color-border)",
                        borderRadius: 8,
                        fontSize: 12,
                      }}
                    />
                    <Area type="monotone" dataKey="Stock In" stroke="var(--color-health)" strokeWidth={2} fill="url(#gIn)" />
                    <Area type="monotone" dataKey="Stock Out" stroke="var(--color-critical)" strokeWidth={2} fill="url(#gOut)" />
                  </AreaChart>
                </ResponsiveContainer>
              </div>
            ) : (
              <EmptyState title="No movement yet" description="Stock transactions will appear here." />
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Category Distribution</CardTitle>
            <CardDescription>Products per category</CardDescription>
          </CardHeader>
          <CardContent>
            {category.isLoading ? (
              <SkeletonList rows={4} />
            ) : category.isError ? (
              <ErrorState description="Could not load categories." onRetry={() => category.refetch()} />
            ) : category.data?.datasets?.[0]?.data?.some((v) => v > 0) ? (
              <div className="h-72">
                <ResponsiveContainer width="100%" height="100%">
                  <BarChart data={category.data.labels.map((label, i) => ({ name: label, count: category.data!.datasets[0].data[i] ?? 0 }))} margin={{ top: 8, right: 8, bottom: 0, left: -12 }}>
                    <CartesianGrid strokeDasharray="3 3" stroke="var(--color-border)" vertical={false} />
                    <XAxis dataKey="name" tick={{ fontSize: 11, fill: "var(--color-muted-foreground)" }} tickLine={false} axisLine={false} interval={0} angle={-20} textAnchor="end" height={48} />
                    <YAxis tick={{ fontSize: 11, fill: "var(--color-muted-foreground)" }} tickLine={false} axisLine={false} allowDecimals={false} />
                    <Tooltip
                      cursor={{ fill: "var(--color-muted)" }}
                      contentStyle={{
                        backgroundColor: "var(--color-card)",
                        border: "1px solid var(--color-border)",
                        borderRadius: 8,
                        fontSize: 12,
                      }}
                    />
                    <Bar dataKey="count" fill="var(--color-primary)" radius={[4, 4, 0, 0]} maxBarSize={36} />
                  </BarChart>
                </ResponsiveContainer>
              </div>
            ) : (
              <EmptyState title="No categories yet" />
            )}
          </CardContent>
        </Card>
      </div>

      {/* Bottom row */}
      <div className="grid grid-cols-1 gap-4 xl:grid-cols-3">
        {/* Top selling */}
        <Card>
          <CardHeader>
            <CardTitle>Top Selling Products</CardTitle>
            <CardDescription>Units sold across all time</CardDescription>
          </CardHeader>
          <CardContent>
            {topSelling.isLoading ? (
              <SkeletonList rows={4} />
            ) : topSelling.isError ? (
              <ErrorState description="Could not load top sellers." onRetry={() => topSelling.refetch()} />
            ) : topSelling.data?.datasets?.[0]?.data?.some((v) => v > 0) ? (
              <div className="h-64">
                <ResponsiveContainer width="100%" height="100%">
                  <BarChart data={topSelling.data.labels.map((label, i) => ({ name: label, units: topSelling.data!.datasets[0].data[i] ?? 0 }))} layout="vertical" margin={{ top: 4, right: 8, bottom: 0, left: 8 }}>
                    <CartesianGrid strokeDasharray="3 3" stroke="var(--color-border)" horizontal={false} />
                    <XAxis type="number" tick={{ fontSize: 11, fill: "var(--color-muted-foreground)" }} tickLine={false} axisLine={false} allowDecimals={false} />
                    <YAxis type="category" dataKey="name" tick={{ fontSize: 11, fill: "var(--color-muted-foreground)" }} tickLine={false} axisLine={false} width={110} />
                    <Tooltip
                      cursor={{ fill: "var(--color-muted)" }}
                      contentStyle={{
                        backgroundColor: "var(--color-card)",
                        border: "1px solid var(--color-border)",
                        borderRadius: 8,
                        fontSize: 12,
                      }}
                    />
                    <Bar dataKey="units" fill="var(--color-info)" radius={[0, 4, 4, 0]} maxBarSize={20} />
                  </BarChart>
                </ResponsiveContainer>
              </div>
            ) : (
              <EmptyState title="No sales yet" description="Stock-out transactions will rank products here." />
            )}
          </CardContent>
        </Card>

        {/* Recent activity */}
        <Card>
          <CardHeader className="flex-row items-start justify-between space-y-0">
            <div>
              <CardTitle>Recent Activity</CardTitle>
              <CardDescription>Latest actions across the system</CardDescription>
            </div>
            {isAdmin && (
              <Button variant="ghost" size="sm" className="text-xs" asChild>
                <Link to="/activity">View all</Link>
              </Button>
            )}
          </CardHeader>
          <CardContent>
            {summary.isLoading ? (
              <SkeletonList rows={5} />
            ) : summary.isError ? (
              <ErrorState description="Could not load activity." onRetry={() => summary.refetch()} />
            ) : summaryData?.recent_activities?.length ? (
              <ul className="space-y-3">
                {summaryData.recent_activities.slice(0, 6).map((a) => (
                  <ActivityRow key={a.id} action={a.action} userName={a.user_name} time={a.created_at} />
                ))}
              </ul>
            ) : (
              <EmptyState title="No activity yet" />
            )}
          </CardContent>
        </Card>

        {/* Low stock */}
        <Card>
          <CardHeader>
            <CardTitle>Low Stock Alerts</CardTitle>
            <CardDescription>Items at or below their threshold</CardDescription>
          </CardHeader>
          <CardContent>
            {summary.isLoading ? (
              <SkeletonList rows={4} />
            ) : summary.isError ? (
              <ErrorState description="Could not load stock alerts." onRetry={() => summary.refetch()} />
            ) : summaryData?.low_stock_items?.length ? (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Product</TableHead>
                    <TableHead className="text-right">On hand</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {summaryData.low_stock_items.slice(0, 5).map((item) => (
                    <TableRow key={item.product_id}>
                      <TableCell>
                        <p className="truncate font-medium">{item.name}</p>
                        <p className="text-xs text-muted-foreground">{item.sku}</p>
                      </TableCell>
                      <TableCell className="text-right">
                        <Badge variant={item.quantity === 0 ? "critical" : "warning"}>
                          {item.quantity} left
                        </Badge>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            ) : (
              <EmptyState
                title="All stock healthy"
                description="No products are below their restock threshold."
              />
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  );
}

function KpiCard({
  label,
  value,
  icon: Icon,
  tone = "default",
}: {
  label: string;
  value?: string;
  icon: React.ComponentType<{ className?: string }>;
  tone?: "default" | "warning";
}) {
  return (
    <Card>
      <CardContent className="flex items-start justify-between">
        <div>
          <p className="text-sm text-muted-foreground">{label}</p>
          {value === undefined ? (
            <div className="mt-2 h-8 w-24 animate-pulse rounded bg-muted" />
          ) : (
            <p
              className={cn(
                "mt-1 text-2xl font-semibold tracking-tight",
                tone === "warning" && "text-warning-foreground",
              )}
            >
              {value}
            </p>
          )}
        </div>
        <div
          className={cn(
            "flex h-10 w-10 shrink-0 items-center justify-center rounded-lg",
            tone === "warning" ? "bg-warning/15 text-warning" : "bg-primary/10 text-primary",
          )}
        >
          <Icon className="h-5 w-5" />
        </div>
      </CardContent>
    </Card>
  );
}

function ActivityRow({ action, userName, time }: { action: string; userName: string; time: string }) {
  const icon =
    action === "LOGIN" ? (
      <Clock className="h-4 w-4" />
    ) : action === "LOGOUT" ? (
      <Clock className="h-4 w-4" />
    ) : action === "REFRESH" ? (
      <PackagePlus className="h-4 w-4" />
    ) : (
      <ClipboardList className="h-4 w-4" />
    );

  return (
    <li className="flex items-start gap-3">
      <div className="mt-0.5 flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-muted text-muted-foreground">
        {icon}
      </div>
      <div className="min-w-0 flex-1">
        <p className="truncate text-sm">
          <span className="font-medium">{userName}</span>{" "}
          <span className="text-muted-foreground">{action.toLowerCase().replace(/_/g, " ")}</span>
        </p>
        <p className="text-xs text-muted-foreground">{timeAgo(time)}</p>
      </div>
    </li>
  );
}