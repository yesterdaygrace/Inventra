import * as React from "react";
import { Link } from "react-router-dom";
import {
  Boxes,
  Package,
  DollarSign,
  AlertTriangle,
  ArrowDownToLine,
  ArrowUpFromLine,
  ArrowUpRight,
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
  Cell,
  Pie,
  PieChart,
  ResponsiveContainer,
  Sector,
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
    <div className="space-y-8">
      {/* Header with more presence */}
      <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-3xl font-semibold tracking-tight sm:text-4xl">
            Dashboard
          </h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Welcome back,{" "}
            <span className="font-medium text-foreground">
              {user?.name.split(" ")[0] ?? "there"}
            </span>
            . Here is what is happening in your inventory.
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

      {/* KPI cards — staggered entry, each card navigates to its module */}
      <div className="rise-stagger grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <KpiCard
          label="Total Products"
          value={summary.isLoading ? undefined : formatNumber(summaryData?.total_products)}
          icon={Package}
          tone="default"
          to="/products"
        />
        <KpiCard
          label="Total Categories"
          value={summary.isLoading ? undefined : formatNumber(summaryData?.total_categories)}
          icon={Boxes}
          tone="default"
          to="/categories"
        />
        <KpiCard
          label="Inventory Value"
          value={summary.isLoading ? undefined : formatCurrency(summaryData?.inventory_value)}
          icon={DollarSign}
          tone="default"
          to="/inventory"
        />
        <KpiCard
          label="Low Stock Items"
          value={summary.isLoading ? undefined : formatNumber(summaryData?.low_stock_count)}
          icon={AlertTriangle}
          tone={summaryData && summaryData.low_stock_count > 0 ? "warning" : "default"}
          to="/inventory?low_stock=true"
        />
      </div>

      {/* Charts row */}
      <div className="grid grid-cols-1 gap-6 xl:grid-cols-3">
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
                    <XAxis dataKey="name" tick={AXIS_TICK} tickLine={false} axisLine={false} interval="preserveStartEnd" minTickGap={32} />
                    <YAxis tick={AXIS_TICK} tickLine={false} axisLine={false} width={48} tickFormatter={compactNumber} />
                    <Tooltip
                      content={<ChartTooltip />}
                      cursor={{ stroke: "var(--color-muted-foreground)", strokeDasharray: "4 4" }}
                    />
                    <Area type="monotone" dataKey="Stock In" stroke="var(--color-health)" strokeWidth={2} fill="url(#gIn)" activeDot={{ r: 4, strokeWidth: 2 }} />
                    <Area type="monotone" dataKey="Stock Out" stroke="var(--color-critical)" strokeWidth={2} fill="url(#gOut)" activeDot={{ r: 4, strokeWidth: 2 }} />
                  </AreaChart>
                </ResponsiveContainer>
                <div className="mt-2 flex items-center justify-center gap-5 text-xs text-muted-foreground">
                  <span className="flex items-center gap-1.5">
                    <span aria-hidden className="h-2 w-2 rounded-full bg-health" />
                    Stock In
                  </span>
                  <span className="flex items-center gap-1.5">
                    <span aria-hidden className="h-2 w-2 rounded-full bg-critical" />
                    Stock Out
                  </span>
                </div>
              </div>
            ) : (
              <EmptyState title="No movement yet" description="Stock transactions will appear here." />
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Category Distribution</CardTitle>
            <CardDescription>Top 8 categories by product count</CardDescription>
          </CardHeader>
          <CardContent>
            {category.isLoading ? (
              <SkeletonList rows={4} />
            ) : category.isError ? (
              <ErrorState description="Could not load categories." onRetry={() => category.refetch()} />
            ) : category.data?.datasets?.[0]?.data?.some((v) => v > 0) ? (
              <CategoryPie
                data={category.data.labels
                  .map((label, i) => ({
                    name: label,
                    value: category.data!.datasets[0].data[i] ?? 0,
                  }))
                  .filter((d) => d.value > 0)
                  .sort((a, b) => b.value - a.value)
                  .slice(0, 8)}
              />
            ) : (
              <EmptyState title="No categories yet" />
            )}
          </CardContent>
        </Card>
      </div>

      {/* Bottom row */}
      <div className="grid grid-cols-1 gap-6 xl:grid-cols-3">
        {/* Top selling — now interactive: hover highlight, click to view products */}
        <Card>
          <CardHeader>
            <CardTitle>Top Selling Products</CardTitle>
            <CardDescription>Units sold across all time · click a bar to explore</CardDescription>
          </CardHeader>
          <CardContent>
            {topSelling.isLoading ? (
              <SkeletonList rows={4} />
            ) : topSelling.isError ? (
              <ErrorState description="Could not load top sellers." onRetry={() => topSelling.refetch()} />
            ) : topSelling.data?.datasets?.[0]?.data?.some((v) => v > 0) ? (
              <TopSellingChart
                data={topSelling.data.labels.map((label, i) => ({
                  name: label,
                  units: topSelling.data!.datasets[0].data[i] ?? 0,
                }))}
              />
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
                    <TableRow key={item.product_id} className="cursor-pointer">
                      <TableCell>
                        <Link
                          to="/inventory"
                          className="block rounded-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                        >
                          <p className="truncate font-medium hover:underline">{item.name}</p>
                          <p className="text-xs text-muted-foreground">{item.sku}</p>
                        </Link>
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

// Compact axis number format: 1200 -> "1.2k", 950 -> "950"
function compactNumber(v: number): string {
  if (Math.abs(v) >= 1000) {
    const s = (v / 1000).toFixed(1).replace(/\.0$/, "");
    return `${s}k`;
  }
  return String(v);
}

// Truncate long category/product names for axis ticks
function truncateLabel(v: string): string {
  return v.length > 18 ? `${v.slice(0, 17)}…` : v;
}

interface TooltipPayloadEntry {
  name?: string | number;
  value?: number | string;
  color?: string;
}

// Shared card-styled tooltip: muted label header, one dot-labeled row per series.
function ChartTooltip({
  active,
  payload,
  label,
  unit,
}: {
  active?: boolean;
  payload?: TooltipPayloadEntry[];
  label?: string | number;
  unit?: string;
}) {
  if (!active || !payload?.length) return null;
  return (
    <div className="min-w-36 rounded-lg border border-border bg-card px-3 py-2 shadow-md">
      {label !== undefined && label !== "" && (
        <p className="mb-1.5 truncate text-xs font-medium text-muted-foreground">{label}</p>
      )}
      <div className="space-y-1">
        {payload.map((entry, i) => (
          <div key={i} className="flex items-center gap-2 text-xs">
            <span
              aria-hidden
              className="h-2 w-2 shrink-0 rounded-full"
              style={{ backgroundColor: entry.color }}
            />
            <span className="truncate text-muted-foreground">{entry.name}</span>
            <span className="ml-auto pl-4 font-medium tabular-nums text-foreground">
              {typeof entry.value === "number" ? entry.value.toLocaleString() : entry.value}
              {unit ? ` ${unit}` : ""}
            </span>
          </div>
        ))}
      </div>
    </div>
  );
}

const AXIS_TICK = { fontSize: 12, fill: "var(--color-muted-foreground)" };

// ——— Pie palette: distinct per category, teal-anchored ———
const PIE_COLORS = [
  "var(--color-primary)",
  "oklch(0.58 0.11 235)",
  "oklch(0.62 0.12 165)",
  "oklch(0.68 0.10 40)",
  "oklch(0.60 0.12 285)",
  "oklch(0.55 0.13 25)",
  "oklch(0.66 0.09 210)",
  "oklch(0.72 0.08 145)",
] as const;

function PieTooltip({ active, payload, total }: { active?: boolean; payload?: Array<{ payload: { name: string; value: number }; color?: string }>; total: number }) {
  if (!active || !payload?.length) return null;
  const entry = payload[0];
  const { name, value } = entry.payload;
  const pct = total > 0 ? ((value / total) * 100).toFixed(1) : "0";
  return (
    <div className="min-w-36 rounded-lg border border-border bg-card px-3 py-2 shadow-md">
      <p className="mb-1.5 flex items-center gap-2 truncate text-xs font-medium text-muted-foreground">
        <span aria-hidden className="h-2 w-2 shrink-0 rounded-full" style={{ backgroundColor: entry.color }} />
        {name}
      </p>
      <p className="text-xs">
        <span className="font-medium tabular-nums text-foreground">{value.toLocaleString()} products</span>
        <span className="ml-2 tabular-nums text-muted-foreground">{pct}%</span>
      </p>
    </div>
  );
}

// Recharts active shape for donut: outer ring bump + centered label
function renderPieActiveShape(props: unknown) {
  const p = props as {
    cx: number; cy: number;
    innerRadius: number; outerRadius: number;
    startAngle: number; endAngle: number;
    fill: string; payload: { name: string; value: number };
  };
  return (
    <g>
      <Sector cx={p.cx} cy={p.cy} innerRadius={p.innerRadius} outerRadius={p.outerRadius + 6} startAngle={p.startAngle} endAngle={p.endAngle} fill={p.fill} stroke="var(--color-card)" strokeWidth={2} />
      <Sector cx={p.cx} cy={p.cy} innerRadius={p.outerRadius + 8} outerRadius={p.outerRadius + 10} startAngle={p.startAngle} endAngle={p.endAngle} fill={p.fill} opacity={0.35} />
    </g>
  );
}

function CategoryPie({ data }: { data: Array<{ name: string; value: number }> }) {
  const [activeIndex, setActiveIndex] = React.useState(0);
  const total = React.useMemo(() => data.reduce((s, d) => s + d.value, 0), [data]);

  return (
    <div className="flex flex-col">
      <div className="h-56">
        <ResponsiveContainer width="100%" height="100%">
          <PieChart>
            <Pie
              data={data}
              cx="50%"
              cy="50%"
              innerRadius={58}
              outerRadius={88}
              paddingAngle={2}
              dataKey="value"
              nameKey="name"
              isAnimationActive
              animationDuration={600}
              activeIndex={activeIndex}
              activeShape={renderPieActiveShape}
              onMouseEnter={(_, i) => setActiveIndex(i)}
            >
              {data.map((_, i) => (
                <Cell key={`c-${i}`} fill={PIE_COLORS[i % PIE_COLORS.length]} stroke="var(--color-card)" strokeWidth={2} className="transition-opacity hover:opacity-90" />
              ))}
            </Pie>
            <Tooltip content={<PieTooltip total={total} />} />
          </PieChart>
        </ResponsiveContainer>
      </div>
      {/* Custom legend: all 8 rows, keyboard-accessible, dot + label + percent */}
      <div className="mt-2 grid grid-cols-2 gap-x-4 gap-y-1.5 px-1">
        {data.map((d, i) => {
          const pct = total > 0 ? ((d.value / total) * 100).toFixed(0) : "0";
          return (
            <button
              key={d.name}
              type="button"
              onMouseEnter={() => setActiveIndex(i)}
              onFocus={() => setActiveIndex(i)}
              className="flex items-center gap-2 rounded px-1 py-0.5 text-left text-xs transition-colors hover:bg-muted/50 focus-visible:bg-muted/50 focus-visible:outline-none"
            >
              <span aria-hidden className="h-2.5 w-2.5 shrink-0 rounded-full" style={{ backgroundColor: PIE_COLORS[i % PIE_COLORS.length] }} />
              <span className="min-w-0 flex-1 truncate text-muted-foreground" title={d.name}>{d.name}</span>
              <span className="shrink-0 tabular-nums text-foreground">{pct}%</span>
            </button>
          );
        })}
      </div>
    </div>
  );
}

function TopSellingChart({ data }: { data: Array<{ name: string; units: number }> }) {
  const [activeIndex, setActiveIndex] = React.useState<number | null>(null);
  // Only items with sales; sorted longest-on-top for scanability
  const sorted = React.useMemo(() => [...data].filter((d) => d.units > 0).sort((a, b) => b.units - a.units).slice(0, 8), [data]);

  return (
    <div className="h-64">
      <ResponsiveContainer width="100%" height="100%">
        <BarChart
          data={sorted}
          layout="vertical"
          margin={{ top: 4, right: 16, bottom: 0, left: 8 }}
          onMouseMove={(state: unknown) => {
            const s = state as { activeTooltipIndex?: number } | null;
            if (s && typeof s.activeTooltipIndex === "number") setActiveIndex(s.activeTooltipIndex);
          }}
          onMouseLeave={() => setActiveIndex(null)}
        >
          <CartesianGrid strokeDasharray="3 3" stroke="var(--color-border)" horizontal={false} />
          <XAxis type="number" tick={AXIS_TICK} tickLine={false} axisLine={false} allowDecimals={false} tickFormatter={compactNumber} />
          <YAxis type="category" dataKey="name" tick={AXIS_TICK} tickLine={false} axisLine={false} width={140} tickFormatter={truncateLabel} />
          <Tooltip content={<ChartTooltip unit="units" />} cursor={{ fill: "var(--color-muted)" }} />
          <Bar dataKey="units" name="Sold" radius={[0, 4, 4, 0]} maxBarSize={22} isAnimationActive animationDuration={700} cursor="pointer">
            {sorted.map((_, i) => (
              <Cell
                key={`b-${i}`}
                fill={activeIndex === i ? "var(--color-primary)" : "var(--color-info)"}
                opacity={activeIndex === null ? 1 : activeIndex === i ? 1 : 0.55}
                className="transition-all duration-150"
              />
            ))}
          </Bar>
        </BarChart>
      </ResponsiveContainer>
      <p className="mt-2 text-center text-[11px] text-muted-foreground">Hover to highlight · values animate on load</p>
    </div>
  );
}

function KpiCard({
  label,
  value,
  icon: Icon,
  tone = "default",
  to,
}: {
  label: string;
  value?: string;
  icon: React.ComponentType<{ className?: string }>;
  tone?: "default" | "warning";
  to?: string;
}) {
  const body = (
    <CardContent className="flex items-start justify-between pt-6">
      <div>
        <p className="text-sm text-muted-foreground">{label}</p>
        {value === undefined ? (
          <div className="mt-2 h-8 w-24 animate-pulse rounded bg-muted" />
        ) : (
          <p
            className={cn(
              "mt-1 text-2xl font-medium tracking-tight",
              tone === "warning" && "text-warning-foreground",
            )}
          >
            {value}
          </p>
        )}
      </div>
      <div className="flex flex-col items-end gap-1">
        <div
          className={cn(
            "flex h-10 w-10 shrink-0 items-center justify-center rounded-lg",
            tone === "warning" ? "bg-warning/15 text-warning" : "bg-primary/10 text-primary",
          )}
        >
          <Icon className="h-5 w-5" />
        </div>
        {to && (
          <ArrowUpRight
            aria-hidden
            className="h-4 w-4 text-muted-foreground opacity-0 transition-opacity duration-150 group-hover:opacity-100"
          />
        )}
      </div>
    </CardContent>
  );

  const hover = to
    ? "transition-[transform,box-shadow] duration-150 [transition-timing-function:var(--ease-out-quad)] group-hover:-translate-y-0.5 group-hover:shadow-md group-focus-visible:ring-2 group-focus-visible:ring-ring"
    : "";

  if (!to) return <Card>{body}</Card>;
  return (
    <Link to={to} className="group rounded-xl focus-visible:outline-none" aria-label={`${label} — open`}>
      <Card className={hover}>{body}</Card>
    </Link>
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
    <li className="-mx-2 flex items-start gap-3 rounded-lg px-2 py-1.5 transition-colors duration-150 hover:bg-muted/50">
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
