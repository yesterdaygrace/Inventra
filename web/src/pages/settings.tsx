import * as React from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { Loader2, CheckCircle } from "lucide-react";
import {
  Button,
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
  Input,
  Label,
  useToast,
} from "@/components/ui";
import { authApi, isApiError } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import type { User } from "@/types/api";

const profileSchema = z.object({
  name: z.string().min(2, "Name must be at least 2 characters"),
  email: z.string().email("Enter a valid email"),
});

const passwordSchema = z
  .object({
    old_password: z.string().min(8, "Enter your current password"),
    new_password: z.string().min(8, "New password must be at least 8 characters"),
    confirm_password: z.string().min(8, "Confirm your new password"),
  })
  .refine((v) => v.new_password === v.confirm_password, {
    message: "Passwords do not match",
    path: ["confirm_password"],
  });

type ProfileValues = z.infer<typeof profileSchema>;
type PasswordValues = z.infer<typeof passwordSchema>;

export function SettingsPage() {
  const { user, setUser } = useAuth();

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">Settings</h1>
        <p className="text-sm text-muted-foreground">Manage your account</p>
      </div>

      <ProfileCard user={user} setUser={setUser} />
      <PasswordCard />
    </div>
  );
}

function ProfileCard({
  user,
  setUser,
}: {
  user: Pick<User, "name" | "email"> | null;
  setUser: (u: User) => void;
}) {
  const { toast } = useToast();
  const [saving, setSaving] = React.useState(false);
  const [success, setSuccess] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);

  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<ProfileValues>({
    resolver: zodResolver(profileSchema),
    values: { name: user?.name ?? "", email: user?.email ?? "" },
  });

  const submit = handleSubmit(async (values) => {
    setSaving(true);
    setSuccess(false);
    setError(null);
    try {
      const updated = await authApi.updateProfile({
        name: values.name.trim(),
        email: values.email.trim(),
      });
      setUser(updated);
      setSuccess(true);
      toast({ title: "Profile updated", variant: "success" });
      setTimeout(() => setSuccess(false), 3000);
    } catch (err) {
      setError(isApiError(err) ? err.message : "Could not update profile.");
    } finally {
      setSaving(false);
    }
  });

  return (
    <Card>
      <CardHeader>
        <CardTitle>Profile</CardTitle>
        <CardDescription>Update your name and email</CardDescription>
      </CardHeader>
      <CardContent>
        <form onSubmit={submit} className="space-y-4" noValidate>
          {error && (
            <div
              role="alert"
              className="rounded-md border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive"
            >
              {error}
            </div>
          )}

          {success && (
            <div className="flex items-center gap-2 text-sm text-health">
              <CheckCircle className="h-4 w-4" />
              Saved
            </div>
          )}

          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <div className="space-y-2">
              <Label htmlFor="profile-name">Name</Label>
              <Input id="profile-name" error={!!errors.name} {...register("name")} />
              {errors.name && <p className="text-xs text-destructive">{errors.name.message}</p>}
            </div>
            <div className="space-y-2">
              <Label htmlFor="profile-email">Email</Label>
              <Input id="profile-email" type="email" error={!!errors.email} {...register("email")} />
              {errors.email && <p className="text-xs text-destructive">{errors.email.message}</p>}
            </div>
          </div>

          <Button type="submit" disabled={saving}>
            {saving && <Loader2 className="animate-spin" />}
            Save changes
          </Button>
        </form>
      </CardContent>
    </Card>
  );
}

function PasswordCard() {
  const { toast } = useToast();
  const [saving, setSaving] = React.useState(false);
  const [success, setSuccess] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);

  const {
    register,
    handleSubmit,
    reset,
    formState: { errors },
  } = useForm<PasswordValues>({
    resolver: zodResolver(passwordSchema),
    defaultValues: { old_password: "", new_password: "", confirm_password: "" },
  });

  const submit = handleSubmit(async (values) => {
    setSaving(true);
    setSuccess(false);
    setError(null);
    try {
      await authApi.changePassword(values.old_password, values.new_password);
      reset();
      setSuccess(true);
      toast({ title: "Password changed", variant: "success" });
      setTimeout(() => setSuccess(false), 3000);
    } catch (err) {
      setError(isApiError(err) ? err.message : "Could not change password.");
    } finally {
      setSaving(false);
    }
  });

  return (
    <Card>
      <CardHeader>
        <CardTitle>Change password</CardTitle>
        <CardDescription>Enter your current password and choose a new one</CardDescription>
      </CardHeader>
      <CardContent>
        <form onSubmit={submit} className="space-y-4" noValidate>
          {error && (
            <div
              role="alert"
              className="rounded-md border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive"
            >
              {error}
            </div>
          )}

          {success && (
            <div className="flex items-center gap-2 text-sm text-health">
              <CheckCircle className="h-4 w-4" />
              Password changed
            </div>
          )}

          <div className="space-y-2">
            <Label htmlFor="pwd-current">Current password</Label>
            <Input id="pwd-current" type="password" error={!!errors.old_password} {...register("old_password")} />
            {errors.old_password && <p className="text-xs text-destructive">{errors.old_password.message}</p>}
          </div>

          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <div className="space-y-2">
              <Label htmlFor="pwd-new">New password</Label>
              <Input id="pwd-new" type="password" error={!!errors.new_password} {...register("new_password")} />
              {errors.new_password && <p className="text-xs text-destructive">{errors.new_password.message}</p>}
            </div>
            <div className="space-y-2">
              <Label htmlFor="pwd-confirm">Confirm password</Label>
              <Input id="pwd-confirm" type="password" error={!!errors.confirm_password} {...register("confirm_password")} />
              {errors.confirm_password && (
                <p className="text-xs text-destructive">{errors.confirm_password.message}</p>
              )}
            </div>
          </div>

          <Button type="submit" disabled={saving}>
            {saving && <Loader2 className="animate-spin" />}
            Change password
          </Button>
        </form>
      </CardContent>
    </Card>
  );
}