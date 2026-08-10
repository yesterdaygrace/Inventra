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
  Select,
} from "@/components/ui";
import type { Product, TransferRequest, Warehouse } from "@/types/api";
import { isApiError } from "@/lib/api";

const transferSchema = z.object({
  product_id: z.string().min(1, "Select a product"),
  from_warehouse_id: z.string().min(1, "Select source warehouse"),
  to_warehouse_id: z.string().min(1, "Select destination warehouse"),
  quantity: z.coerce.number({ invalid_type_error: "Enter a quantity" }).min(1, "Quantity must be at least 1"),
  note: z.string().max(500).optional().or(z.literal("")),
});

export type TransferValues = z.infer<typeof transferSchema>;

export function TransferDialog({
  open,
  onOpenChange,
  products,
  warehouses,
  initialProductId,
  onSubmit,
  submitting,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  products: Product[];
  warehouses: Warehouse[];
  initialProductId?: string;
  onSubmit: (values: TransferRequest) => Promise<void>;
  submitting: boolean;
}) {
  const {
    register,
    handleSubmit,
    reset,
    formState: { errors },
  } = useForm<TransferValues>({
    resolver: zodResolver(transferSchema),
    values: {
      product_id: initialProductId ?? "",
      from_warehouse_id: "",
      to_warehouse_id: "",
      quantity: 1,
      note: "",
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
    if (values.from_warehouse_id === values.to_warehouse_id) {
      setFormError("Source and destination warehouse must be different.");
      return;
    }
    setFormError(null);
    try {
      await onSubmit({
        product_id: values.product_id,
        from_warehouse_id: values.from_warehouse_id,
        to_warehouse_id: values.to_warehouse_id,
        quantity: values.quantity,
        note: values.note?.trim() || undefined,
      });
      reset();
      onOpenChange(false);
    } catch (err) {
      setFormError(isApiError(err) ? err.message : "Could not transfer the item.");
    }
  });

  const activeWarehouses = warehouses.filter((w) => w.is_active !== false);

  return (
    <Dialog open={open} onOpenChange={onClose}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Transfer stock</DialogTitle>
          <DialogDescription>Move stock from one warehouse to another.</DialogDescription>
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
            <Label htmlFor="transfer-product">Product</Label>
            <Select
              id="transfer-product"
              placeholder="Select a product"
              error={!!errors.product_id}
              options={products.map((p) => ({ value: p.id, label: `${p.name} (${p.sku})` }))}
              {...register("product_id")}
            />
            {errors.product_id && <p className="text-xs text-destructive">{errors.product_id.message}</p>}
          </div>

          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <div className="space-y-2">
              <Label htmlFor="transfer-from">From warehouse</Label>
              <Select
                id="transfer-from"
                placeholder="Select source"
                error={!!errors.from_warehouse_id}
                options={activeWarehouses.map((w) => ({ value: w.id, label: `${w.name} (${w.code})` }))}
                {...register("from_warehouse_id")}
              />
              {errors.from_warehouse_id && (
                <p className="text-xs text-destructive">{errors.from_warehouse_id.message}</p>
              )}
            </div>

            <div className="space-y-2">
              <Label htmlFor="transfer-to">To warehouse</Label>
              <Select
                id="transfer-to"
                placeholder="Select destination"
                error={!!errors.to_warehouse_id}
                options={activeWarehouses.map((w) => ({ value: w.id, label: `${w.name} (${w.code})` }))}
                {...register("to_warehouse_id")}
              />
              {errors.to_warehouse_id && (
                <p className="text-xs text-destructive">{errors.to_warehouse_id.message}</p>
              )}
            </div>
          </div>

          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <div className="space-y-2">
              <Label htmlFor="transfer-quantity">Quantity</Label>
              <Input
                id="transfer-quantity"
                type="number"
                min="1"
                step="1"
                error={!!errors.quantity}
                {...register("quantity")}
              />
              {errors.quantity && <p className="text-xs text-destructive">{errors.quantity.message}</p>}
            </div>

            <div className="space-y-2">
              <Label htmlFor="transfer-note">Note (optional)</Label>
              <Input
                id="transfer-note"
                placeholder="Optional note"
                error={!!errors.note}
                {...register("note")}
              />
              {errors.note && <p className="text-xs text-destructive">{errors.note.message}</p>}
            </div>
          </div>

          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onClose(false)} disabled={submitting}>
              Cancel
            </Button>
            <Button type="submit" disabled={submitting}>
              {submitting && <Loader2 className="animate-spin" />}
              Transfer
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}