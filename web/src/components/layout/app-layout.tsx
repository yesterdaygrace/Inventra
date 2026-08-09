import * as React from "react";
import { NavLink, Outlet, useLocation, useNavigate } from "react-router-dom";
import {
  LayoutDashboard,
  Package,
  Tags,
  Boxes,
  ArrowLeftRight,
  BarChart3,
  ClipboardList,
  Users,
  Settings,
  Search,
  Sun,
  Moon,
  LogOut,
  Menu,
  PanelLeft,
  ChevronDown,
  type LucideIcon,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui";
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
} from "@/components/ui";
import { useAuth } from "@/lib/auth";
import { useTheme } from "@/lib/theme";
import { initials } from "@/lib/format";
import { CommandPalette, type CommandItem } from "@/components/layout/command-palette";

const NAV_ITEMS: CommandItem[] = [
  { label: "Dashboard", to: "/", icon: LayoutDashboard, shortcut: "G D" },
  { label: "Products", to: "/products", icon: Package, shortcut: "G P" },
  { label: "Categories", to: "/categories", icon: Tags, shortcut: "G C" },
  { label: "Inventory", to: "/inventory", icon: Boxes, shortcut: "G I" },
  { label: "Transactions", to: "/transactions", icon: ArrowLeftRight, shortcut: "G T" },
  { label: "Reports", to: "/reports", icon: BarChart3, shortcut: "G R" },
  { label: "Activity Log", to: "/activity", icon: ClipboardList, adminOnly: true },
  { label: "Users", to: "/users", icon: Users, adminOnly: true, shortcut: "G U" },
  { label: "Settings", to: "/settings", icon: Settings, shortcut: "G S" },
];

export function AppLayout() {
  const [sidebarCollapsed, setSidebarCollapsed] = React.useState(false);
  const [mobileOpen, setMobileOpen] = React.useState(false);
  const [paletteOpen, setPaletteOpen] = React.useState(false);
  const { user } = useAuth();
  const { theme, setTheme } = useTheme();

  React.useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        setPaletteOpen((v) => !v);
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  const visibleNav = NAV_ITEMS.filter((it) => (it.adminOnly ? user?.role === "ADMIN" : true));

  return (
    <div className="min-h-screen bg-background">
      {/* Desktop sidebar */}
      <aside
        className={cn(
          "fixed inset-y-0 left-0 z-40 hidden border-r border-border bg-card transition-all duration-200 xl:flex xl:flex-col",
          sidebarCollapsed ? "xl:w-16" : "xl:w-60",
        )}
      >
        <div className={cn("flex h-14 items-center gap-2 border-b border-border px-4", sidebarCollapsed && "xl:justify-center xl:px-0")}>
          <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-primary text-sm font-bold text-primary-foreground">
            I
          </div>
          {!sidebarCollapsed && (
            <span className="truncate text-base font-semibold tracking-tight">Inventra</span>
          )}
        </div>

        <nav className="flex-1 space-y-1 overflow-y-auto p-3">
          {visibleNav.map((item) => (
            <SidebarLink key={item.to} item={item} collapsed={sidebarCollapsed} onNavigate={() => setMobileOpen(false)} />
          ))}
        </nav>

        <div className="border-t border-border p-3">
          <UserMenu collapsed={sidebarCollapsed} />
        </div>
      </aside>

      {/* Mobile drawer */}
      {mobileOpen && (
        <div className="fixed inset-0 z-50 xl:hidden">
          <div className="absolute inset-0 bg-black/60" onClick={() => setMobileOpen(false)} />
          <aside className="absolute inset-y-0 left-0 flex w-64 flex-col bg-card shadow-xl">
            <div className="flex h-14 items-center gap-2 border-b border-border px-4">
              <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-primary text-sm font-bold text-primary-foreground">
                I
              </div>
              <span className="text-base font-semibold tracking-tight">Inventra</span>
            </div>
            <nav className="flex-1 space-y-1 overflow-y-auto p-3">
              {visibleNav.map((item) => (
                <SidebarLink key={item.to} item={item} collapsed={false} onNavigate={() => setMobileOpen(false)} />
              ))}
            </nav>
            <div className="border-t border-border p-3">
              <UserMenu collapsed={false} />
            </div>
          </aside>
        </div>
      )}

      {/* Main column */}
      <div className={cn("flex min-h-screen flex-col transition-all duration-200", sidebarCollapsed ? "xl:pl-16" : "xl:pl-60")}>
        <header className="sticky top-0 z-30 flex h-14 items-center gap-2 border-b border-border bg-background/95 px-4 backdrop-blur">
          <Button
            variant="ghost"
            size="icon"
            className="xl:hidden"
            onClick={() => setMobileOpen(true)}
            aria-label="Open navigation"
          >
            <Menu className="h-5 w-5" />
          </Button>
          <Button
            variant="ghost"
            size="icon"
            className="hidden xl:inline-flex"
            onClick={() => setSidebarCollapsed((v) => !v)}
            aria-label="Toggle sidebar"
          >
            <PanelLeft className="h-5 w-5" />
          </Button>

          <div className="hidden items-center gap-2 text-sm text-muted-foreground sm:flex">
            <span className="font-medium text-foreground">Inventra</span>
            <span>/</span>
            <PageTitle items={visibleNav} />
          </div>

          <div className="ml-auto flex items-center gap-1">
            <Button
              variant="ghost"
              size="sm"
              className="hidden gap-2 text-muted-foreground md:inline-flex"
              onClick={() => setPaletteOpen(true)}
            >
              <Search className="h-4 w-4" />
              <span className="text-xs">Search…</span>
              <kbd className="rounded border border-border bg-muted px-1.5 py-0.5 text-[10px]">
                Ctrl K
              </kbd>
            </Button>
            <Button
              variant="ghost"
              size="icon"
              className="md:hidden"
              onClick={() => setPaletteOpen(true)}
              aria-label="Search"
            >
              <Search className="h-5 w-5" />
            </Button>

            <Button
              variant="ghost"
              size="icon"
              onClick={() => setTheme(theme === "dark" ? "light" : "dark")}
              aria-label="Toggle theme"
            >
              {theme === "dark" ? <Sun className="h-5 w-5" /> : <Moon className="h-5 w-5" />}
            </Button>

            <UserMenu collapsed={false} mobile />
          </div>
        </header>

        <main className="flex-1 p-4 md:p-6 lg:p-8">
          <Outlet />
        </main>
      </div>

      <CommandPalette open={paletteOpen} onOpenChange={setPaletteOpen} items={visibleNav} />
    </div>
  );
}

function SidebarLink({
  item,
  collapsed,
  onNavigate,
}: {
  item: CommandItem;
  collapsed: boolean;
  onNavigate?: () => void;
}) {
  return (
    <NavLink
      to={item.to}
      end={item.to === "/"}
      onClick={onNavigate}
      title={collapsed ? item.label : undefined}
      className={({ isActive }) =>
        cn(
          "flex items-center gap-3 rounded-md px-3 py-2 text-sm transition-colors",
          "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring",
          isActive
            ? "bg-accent font-medium text-accent-foreground"
            : "text-muted-foreground hover:bg-accent/60 hover:text-foreground",
          collapsed && "xl:justify-center xl:px-0",
        )
      }
    >
      <item.icon className={cn("h-5 w-5 shrink-0", collapsed && "xl:h-5 xl:w-5")} />
      {!collapsed && <span className="truncate">{item.label}</span>}
    </NavLink>
  );
}

function PageTitle({ items }: { items: CommandItem[] }) {
  const { pathname } = useLocation();
  const match = items.find((it) => it.to === pathname);
  return <span className="capitalize">{match?.label ?? "Page"}</span>;
}

function UserMenu({ collapsed, mobile }: { collapsed: boolean; mobile?: boolean }) {
  const { user, logout } = useAuth();
  const navigate = useNavigate();

  if (!user) return null;

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <button
          className={cn(
            "flex w-full items-center gap-2 rounded-md p-2 text-left text-sm transition-colors hover:bg-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring",
            collapsed && !mobile && "xl:justify-center",
          )}
        >
          <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-primary/15 text-xs font-semibold text-primary">
            {initials(user.name)}
          </span>
          {(!collapsed || mobile) && (
            <span className="min-w-0 flex-1">
              <span className="block truncate text-sm font-medium">{user.name}</span>
              <span className="block truncate text-xs text-muted-foreground">
                {user.role === "ADMIN" ? "Administrator" : "Staff"}
              </span>
            </span>
          )}
          {(!collapsed || mobile) && <ChevronDown className="h-4 w-4 shrink-0 text-muted-foreground" />}
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-56">
        <DropdownMenuLabel className="font-normal">
          <span className="block text-sm font-medium">{user.name}</span>
          <span className="block text-xs text-muted-foreground">{user.email}</span>
        </DropdownMenuLabel>
        <DropdownMenuSeparator />
        <DropdownMenuItem onClick={() => navigate("/settings")}>
          <Settings className="h-4 w-4" />
          Settings
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem variant="destructive" onClick={logout}>
          <LogOut className="h-4 w-4" />
          Sign out
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

export type { LucideIcon };