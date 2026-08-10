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
import type { Warehouse, WarehouseInput } from "@/types/api";
import { isApiError } from "@/lib/api";

const warehouseSchema = z.object({
  code: z.string().min(1, "Code is required").max(50),
  name: z.string().min(1, "Name is required").max(100),
  description: z.string().max(1000).optional().or(z.literal("")),
});

type WarehouseValues = z.infer<typeof warehouseSchema>;

export function WarehouseFormDialog({
  open,
  onOpenChange,
  warehouse,
  onSubmit,
  submitting,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  warehouse?: Warehouse | null;
  onSubmit: (values: WarehouseInput) => Promise<void>;
  submitting: boolean;
}) {
  const isEdit = !!warehouse;

  const {
    register,
    handleSubmit,
    reset,
    formState: { errors },
  } = useForm<WarehouseValues>({
    resolver: zodResolver(warehouseSchema),
    values: {
      code: warehouse?.code ?? "",
      name: warehouse?.name ?? "",
      description: warehouse?.description ?? "",
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
        code: values.code.trim(),
        name: values.name.trim(),
        description: values.description?.trim() || undefined,
      });
      reset();
      onOpenChange(false);
    } catch (err) {
      setFormError(isApiError(err) ? err.message : "Could not save the warehouse.");
    }
  });

  return (
    <Dialog open={open} onOpenChange={onClose}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{isEdit ? "Edit warehouse" : "Add warehouse"}</DialogTitle>
          <DialogDescription>
            {isEdit ? "Update this warehouse's details below." : "Create a new stock location."}
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

          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <div className="space-y-2">
              <Label htmlFor="warehouse-code">Code</Label>
              <Input
                id="warehouse-code"
                placeholder="WH-001"
                error={!!errors.code}
                disabled={isEdit}
                {...register("code")}
              />
              {errors.code && <p className="text-xs text-destructive">{errors.code.message}</p>}
            </div>

            <div className="space-y-2">
              <Label htmlFor="warehouse-name">Name</Label>
              <Input
                id="warehouse-name"
                placeholder="Main warehouse"
                error={!!errors.name}
                {...register("name")}
              />
              {errors.name && <p className="text-xs text-destructive">{errors.name.message}</p>}
            </div>
          </div>

          <div className="space-y-2">
            <Label htmlFor="warehouse-description">Description</Label>
            <Input
              id="warehouse-description"
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
              {isEdit ? "Save changes" : "Create warehouse"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}