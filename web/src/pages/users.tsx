import * as React from "react";
import { Loader2, MoreHorizontal } from "lucide-react";
import {
  Badge,
  Button,
  Card,
  CardContent,
  CardHeader,
  CardTitle,
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
  Input,
  Label,
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
import { userApi } from "@/lib/api";
import { listKeys } from "@/lib/query";
import { useList, useApiMutation, useQueryClient } from "@/hooks/use-query";
import { useAuth } from "@/lib/auth";
import { formatNumber, formatDateTime } from "@/lib/format";
import type { User, Role, RoleUpdateRequest } from "@/types/api";

const PER_PAGE = 10;

const ROLE_OPTIONS = [
  { value: "ADMIN", label: "Admin" },
  { value: "STAFF", label: "Staff" },
];

export function UsersPage() {
  const { user: currentUser } = useAuth();
  const queryClient = useQueryClient();
  const { toast } = useToast();

  const [page, setPage] = React.useState(1);
  const [name, setName] = React.useState("");
  const [debouncedName, setDebouncedName] = React.useState("");
  const [role, setRole] = React.useState<"" | Role>("");
  const [isActive, setIsActive] = React.useState("");
  const [roleDialogUser, setRoleDialogUser] = React.useState<User | null>(null);

  React.useEffect(() => {
    const t = setTimeout(() => {
      setDebouncedName(name);
      setPage(1);
    }, 300);
    return () => clearTimeout(t);
  }, [name]);

  const params = {
    page,
    per_page: PER_PAGE,
    name: debouncedName || undefined,
    role: role || undefined,
    is_active: isActive === "" ? undefined : isActive === "true",
  };

  const users = useList(listKeys.users(params), () => userApi.list(params));

  const deactivate = useApiMutation(
    (id: string) => userApi.deactivate(id),
    {
      onSuccess: () => {
        queryClient.invalidateQueries({ queryKey: ["users"] });
        toast({ title: "User deactivated", variant: "success" });
      },
    },
  );

  const handleDeactivate = (user: User) => {
    if (user.id === currentUser?.id) return;
    if (!window.confirm(`Deactivate ${user.name}?`)) return;
    deactivate.mutate(user.id);
  };

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">Users</h1>
        <p className="text-sm text-muted-foreground">
          User management — {formatNumber(users.data?.pagination?.total)} users
        </p>
      </div>

      <div className="flex flex-wrap items-center gap-2">
        <Input
          className="w-56"
          placeholder="Search name or email…"
          value={name}
          onChange={(e) => setName(e.target.value)}
          aria-label="Search users"
        />
        <Select
          className="w-36"
          aria-label="Filter by role"
          placeholder="All roles"
          value={role}
          onChange={(e) => {
            setRole(e.target.value as "" | Role);
            setPage(1);
          }}
          options={ROLE_OPTIONS}
        />
        <Select
          className="w-36"
          aria-label="Filter by status"
          placeholder="All"
          value={isActive}
          onChange={(e) => {
            setIsActive(e.target.value);
            setPage(1);
          }}
          options={[
            { value: "true", label: "Active" },
            { value: "false", label: "Inactive" },
          ]}
        />
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Users</CardTitle>
        </CardHeader>
        <CardContent>
          {users.isLoading ? (
            <SkeletonList rows={6} />
          ) : users.isError ? (
            <ErrorState description="Could not load users." onRetry={() => users.refetch()} />
          ) : users.data?.items?.length ? (
            <div className="overflow-x-auto">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>User</TableHead>
                    <TableHead>Role</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead>Joined</TableHead>
                    <TableHead className="w-12" />
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {users.data.items.map((u) => (
                    <TableRow key={u.id}>
                      <TableCell>
                        <p className="truncate font-medium">{u.name}</p>
                        <p className="text-xs text-muted-foreground">{u.email}</p>
                      </TableCell>
                      <TableCell>
                        <Badge variant={u.role === "ADMIN" ? "info" : "healthy"}>
                          {u.role}
                        </Badge>
                      </TableCell>
                      <TableCell>
                        <Badge variant={u.is_active ? "healthy" : "secondary"}>
                          {u.is_active ? "Active" : "Inactive"}
                        </Badge>
                      </TableCell>
                      <TableCell className="whitespace-nowrap text-muted-foreground">
                        {formatDateTime(u.created_at)}
                      </TableCell>
                      <TableCell>
                        {u.id !== currentUser?.id && (
                          <DropdownMenu>
                            <DropdownMenuTrigger asChild>
                              <Button variant="ghost" size="icon" className="h-8 w-8" aria-label="Actions">
                                <MoreHorizontal className="h-4 w-4" />
                              </Button>
                            </DropdownMenuTrigger>
                            <DropdownMenuContent align="end">
                              <DropdownMenuItem onClick={() => setRoleDialogUser(u)}>
                                Change role
                              </DropdownMenuItem>
                              <DropdownMenuItem
                                variant="destructive"
                                onClick={() => handleDeactivate(u)}
                              >
                                Deactivate
                              </DropdownMenuItem>
                            </DropdownMenuContent>
                          </DropdownMenu>
                        )}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          ) : (
            <EmptyState title="No users" description="No users match your filters." />
          )}

          {users.data?.pagination && users.data.items.length > 0 && (
            <Pagination
              className="mt-4"
              page={page}
              totalPages={users.data.pagination.total_pages}
              onPageChange={setPage}
            />
          )}
        </CardContent>
      </Card>

      <ChangeRoleDialog
        user={roleDialogUser}
        onClose={() => setRoleDialogUser(null)}
        onSuccess={() => {
          queryClient.invalidateQueries({ queryKey: ["users"] });
          toast({ title: "Role updated", variant: "success" });
        }}
      />
    </div>
  );
}

function ChangeRoleDialog({
  user,
  onClose,
  onSuccess,
}: {
  user: User | null;
  onClose: () => void;
  onSuccess: () => void;
}) {
  const [role, setRole] = React.useState<Role>(user?.role ?? "STAFF");
  const [submitting, setSubmitting] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);

  React.useEffect(() => {
    if (user) {
      setRole(user.role);
      setError(null);
    }
  }, [user]);

  const submit = async () => {
    if (!user) return;
    setSubmitting(true);
    setError(null);
    try {
      await userApi.updateRole(user.id, { role } as RoleUpdateRequest);
      onSuccess();
      onClose();
    } catch (err: unknown) {
      const msg =
        err instanceof Error ? err.message : "Could not update role.";
      setError(msg);
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Dialog open={!!user} onOpenChange={(o) => !o && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Change role</DialogTitle>
        </DialogHeader>

        {error && (
          <div
            role="alert"
            className="rounded-md border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive"
          >
            {error}
          </div>
        )}

        <div className="space-y-2">
          <Label>Role</Label>
          <Select
            aria-label="Role"
            value={role}
            onChange={(e) => setRole(e.target.value as Role)}
            options={ROLE_OPTIONS}
          />
        </div>

        <DialogFooter>
          <Button type="button" variant="outline" onClick={onClose} disabled={submitting}>
            Cancel
          </Button>
          <Button onClick={submit} disabled={submitting}>
            {submitting && <Loader2 className="animate-spin" />}
            Save
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}