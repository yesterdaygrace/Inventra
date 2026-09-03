import * as React from "react";
import { useNavigate } from "react-router-dom";
import { Search, type LucideIcon } from "lucide-react";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui";
import { useAuth } from "@/lib/auth";
import { cn } from "@/lib/utils";

export interface CommandItem {
  label: string;
  to: string;
  icon: LucideIcon;
  keywords?: string[];
  adminOnly?: boolean;
  shortcut?: string;
}

interface CommandPaletteProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  items: CommandItem[];
}

export function CommandPalette({ open, onOpenChange, items }: CommandPaletteProps) {
  const [query, setQuery] = React.useState("");
  const navigate = useNavigate();
  const { user } = useAuth();

  const filtered = React.useMemo(() => {
    const q = query.trim().toLowerCase();
    const visible = items.filter((it) => (it.adminOnly ? user?.role === "ADMIN" : true));
    if (!q) return visible.slice(0, 8);
    return visible
      .filter((it) => `${it.label} ${(it.keywords ?? []).join(" ")}`.toLowerCase().includes(q))
      .slice(0, 8);
  }, [items, query, user]);

  const go = (to: string) => {
    navigate(to);
    onOpenChange(false);
    setQuery("");
  };

  React.useEffect(() => {
    if (open) setQuery("");
  }, [open]);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      {/* Cmd+K is keyboard-initiated and used constantly — no open/close animation. */}
      <DialogContent animation={false} className="sm:max-w-xl p-0 gap-0">
        <DialogHeader className="sr-only">
          <DialogTitle>Quick navigation</DialogTitle>
        </DialogHeader>
        <div className="flex items-center gap-3 border-b border-border px-4">
          <Search className="h-5 w-5 shrink-0 text-muted-foreground" />
          <input
            autoFocus
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Type to search pages…"
            className="h-12 flex-1 bg-transparent text-sm outline-none placeholder:text-muted-foreground"
            onKeyDown={(e) => {
              if (e.key === "Enter" && filtered[0]) go(filtered[0].to);
            }}
          />
          <kbd className="hidden sm:inline-flex items-center rounded border border-border bg-muted px-1.5 py-0.5 text-[10px] text-muted-foreground">
            ESC
          </kbd>
        </div>
        <div className="max-h-72 overflow-auto p-2">
          {filtered.length === 0 ? (
            <p className="px-3 py-8 text-center text-sm text-muted-foreground">
              No matching pages.
            </p>
          ) : (
            filtered.map((it) => (
              <button
                key={it.to}
                onClick={() => go(it.to)}
                className={cn(
                  "flex w-full items-center gap-3 rounded-md px-3 py-2 text-left text-sm",
                  "transition-colors hover:bg-accent hover:text-accent-foreground",
                  "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring",
                )}
              >
                <it.icon className="h-4 w-4 shrink-0 text-muted-foreground" />
                <span className="flex-1">{it.label}</span>
                {it.shortcut && (
                  <kbd className="rounded border border-border bg-muted px-1.5 py-0.5 text-[10px] text-muted-foreground">
                    {it.shortcut}
                  </kbd>
                )}
              </button>
            ))
          )}
        </div>
      </DialogContent>
    </Dialog>
  );
}