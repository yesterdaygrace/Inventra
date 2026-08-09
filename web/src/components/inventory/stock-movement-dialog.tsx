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
import type { Product, StockMovementRequest } from "@/types/api";
import { isApiError } from "@/lib/api";

const movementSchema = z.object({
  product_id: z.string().min(1, "Select a product"),
  quantity: z.coerce.number({ invalid_type_error: "Enter a quantity" }).min(1, "Quantity must be at least 1"),
  unit_cost: z.coerce.number({ invalid_type_error: "Enter a number" }).min(0).optional(),
  note: z.string().max(500).optional().or(z.literal("")),
});

export type MovementValues = z.infer<typeof movementSchema>;
export type MovementType = "IN" | "OUT";

export function StockMovementDialog({
  open,
  onOpenChange,
  movementType,
  products,
  initialProductId,
  onSubmit,
  submitting,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  movementType: MovementType;
  products: Product[];
  initialProductId?: string;
  onSubmit: (values: StockMovementRequest) => Promise<void>;
  submitting: boolean;
}) {
  const isIn = movementType === "IN";

  const {
    register,
    handleSubmit,
    reset,
    formState: { errors },
  } = useForm<MovementValues>({
    resolver: zodResolver(movementSchema),
    values: {
      product_id: initialProductId ?? "",
      quantity: 1,
      unit_cost: undefined,
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
    setFormError(null);
    try {
      await onSubmit({
        product_id: values.product_id,
        quantity: values.quantity,
        unit_cost: values.unit_cost || undefined,
        note: values.note?.trim() || undefined,
      });
      reset();
      onOpenChange(false);
    } catch (err) {
      setFormError(isApiError(err) ? err.message : `Could not ${isIn ? "stock in" : "stock out"} the item.`);
    }
  });

  return (
    <Dialog open={open} onOpenChange={onClose}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{isIn ? "Stock in" : "Stock out"}</DialogTitle>
          <DialogDescription>
            {isIn ? "Add stock to a product." : "Remove stock from a product."}
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
            <Label htmlFor="movement-product">Product</Label>
            <Select
              id="movement-product"
              placeholder="Select a product"
              error={!!errors.product_id}
              options={products.map((p) => ({ value: p.id, label: `${p.name} (${p.sku})` }))}
              {...register("product_id")}
            />
            {errors.product_id && <p className="text-xs text-destructive">{errors.product_id.message}</p>}
          </div>

          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <div className="space-y-2">
              <Label htmlFor="movement-quantity">Quantity</Label>
              <Input
                id="movement-quantity"
                type="number"
                min="1"
                step="1"
                error={!!errors.quantity}
                {...register("quantity")}
              />
              {errors.quantity && <p className="text-xs text-destructive">{errors.quantity.message}</p>}
            </div>

            <div className="space-y-2">
              <Label htmlFor="movement-cost">Unit cost (optional)</Label>
              <Input
                id="movement-cost"
                type="number"
                min="0"
                step="0.01"
                placeholder="0.00"
                error={!!errors.unit_cost}
                {...register("unit_cost")}
              />
              {errors.unit_cost && <p className="text-xs text-destructive">{errors.unit_cost.message}</p>}
            </div>
          </div>

          <div className="space-y-2">
            <Label htmlFor="movement-note">Note (optional)</Label>
            <Input
              id="movement-note"
              placeholder="Optional note"
              error={!!errors.note}
              {...register("note")}
            />
            {errors.note && <p className="text-xs text-destructive">{errors.note.message}</p>}
          </div>

          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onClose(false)} disabled={submitting}>
              Cancel
            </Button>
            <Button type="submit" disabled={submitting} variant={isIn ? "primary" : "danger"}>
              {submitting && <Loader2 className="animate-spin" />}
              {isIn ? "Stock in" : "Stock out"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}