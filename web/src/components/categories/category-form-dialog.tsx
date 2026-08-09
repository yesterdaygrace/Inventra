import * as React from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { Loader2 } from "lucide-react";
import {
  Button,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Input,
  Label,
} from "@/components/ui";
import type { Category, CategoryInput } from "@/types/api";
import { isApiError } from "@/lib/api";

const categorySchema = z.object({
  name: z.string().min(1, "Name is required").max(100),
  description: z.string().max(1000).optional().or(z.literal("")),
});

type CategoryValues = z.infer<typeof categorySchema>;

export function CategoryFormDialog({
  open,
  onOpenChange,
  category,
  onSubmit,
  submitting,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  category?: Category | null;
  onSubmit: (values: CategoryInput) => Promise<void>;
  submitting: boolean;
}) {
  const isEdit = !!category;

  const {
    register,
    handleSubmit,
    reset,
    formState: { errors },
  } = useForm<CategoryValues>({
    resolver: zodResolver(categorySchema),
    values: {
      name: category?.name ?? "",
      description: category?.description ?? "",
    },
  });

  const [formError, setFormError] = React.useState<string | null>(null);

  React.useEffect(() => {
    if (open) setFormError(null);
  }, [open]);

  const onClose = (value: boolean) => {
    if (!submitting) {
      reset();
      onOpenChange(value);
    }
  };

  const submit = handleSubmit(async (values) => {
    setFormError(null);
    try {
      await onSubmit({
        name: values.name.trim(),
        description: values.description?.trim() || undefined,
      });
      reset();
      onOpenChange(false);
    } catch (err) {
      setFormError(isApiError(err) ? err.message : "Could not save the category.");
    }
  });

  return (
    <Dialog open={open} onOpenChange={onClose}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{isEdit ? "Edit category" : "Add category"}</DialogTitle>
          <DialogDescription>
            {isEdit ? "Update this category's details below." : "Create a new product category."}
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={submit} className="space-y-4" noValidate>
          {formError && (
            <div
              role="alert"
              className="rounded-md border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive"
            >
              {formError}
            </div>
          )}

          <div className="space-y-2">
            <Label htmlFor="category-name">Name</Label>
            <Input
              id="category-name"
              placeholder="Electronics"
              error={!!errors.name}
              {...register("name")}
            />
            {errors.name && <p className="text-xs text-destructive">{errors.name.message}</p>}
          </div>

          <div className="space-y-2">
            <Label htmlFor="category-description">Description</Label>
            <Input
              id="category-description"
              placeholder="Optional description"
              error={!!errors.description}
              {...register("description")}
            />
            {errors.description && (
              <p className="text-xs text-destructive">{errors.description.message}</p>
            )}
          </div>

          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onClose(false)} disabled={submitting}>
              Cancel
            </Button>
            <Button type="submit" disabled={submitting}>
              {submitting && <Loader2 className="animate-spin" />}
              {isEdit ? "Save changes" : "Create category"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}