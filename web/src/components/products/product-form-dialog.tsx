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
import type { Category, Product, ProductInput } from "@/types/api";
import { isApiError } from "@/lib/api";

const productSchema = z.object({
  name: z.string().min(1, "Name is required").max(200),
  sku: z.string().min(1, "SKU is required").max(64),
  description: z.string().max(2000).optional().or(z.literal("")),
  price: z.coerce.number({ invalid_type_error: "Enter a price" }).min(0, "Price must be 0 or more"),
  category_id: z.string().min(1, "Category is required"),
  low_stock_threshold: z.coerce
    .number({ invalid_type_error: "Enter a number" })
    .min(0, "Threshold must be 0 or more")
    .optional(),
});

type ProductValues = z.infer<typeof productSchema>;

export function ProductFormDialog({
  open,
  onOpenChange,
  product,
  categories,
  onSubmit,
  submitting,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  product?: Product | null;
  categories: Category[];
  onSubmit: (values: ProductInput) => Promise<void>;
  submitting: boolean;
}) {
  const isEdit = !!product;

  const {
    register,
    handleSubmit,
    reset,
    formState: { errors },
  } = useForm<ProductValues>({
    resolver: zodResolver(productSchema),
    values: {
      name: product?.name ?? "",
      sku: product?.sku ?? "",
      description: product?.description ?? "",
      price: product?.price ?? 0,
      category_id: product?.category_id ?? "",
      low_stock_threshold: product?.low_stock_threshold ?? 10,
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
        sku: values.sku.trim(),
        description: values.description?.trim() || undefined,
        price: values.price,
        category_id: values.category_id,
        low_stock_threshold: values.low_stock_threshold,
      });
      reset();
      onOpenChange(false);
    } catch (err) {
      setFormError(isApiError(err) ? err.message : "Could not save the product.");
    }
  });

  return (
    <Dialog open={open} onOpenChange={onClose}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{isEdit ? "Edit product" : "Add product"}</DialogTitle>
          <DialogDescription>
            {isEdit
              ? "Update the product details below."
              : "Create a new product in your catalog."}
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
            <div className="space-y-2 sm:col-span-2">
              <Label htmlFor="product-name">Name</Label>
              <Input
                id="product-name"
                placeholder="Wireless Keyboard"
                error={!!errors.name}
                {...register("name")}
              />
              {errors.name && <p className="text-xs text-destructive">{errors.name.message}</p>}
            </div>

            <div className="space-y-2">
              <Label htmlFor="product-sku">SKU</Label>
              <Input
                id="product-sku"
                placeholder="KB-1001"
                error={!!errors.sku}
                {...register("sku")}
              />
              {errors.sku && <p className="text-xs text-destructive">{errors.sku.message}</p>}
            </div>

            <div className="space-y-2">
              <Label htmlFor="product-price">Price (USD)</Label>
              <Input
                id="product-price"
                type="number"
                step="0.01"
                min="0"
                placeholder="49.99"
                error={!!errors.price}
                {...register("price")}
              />
              {errors.price && <p className="text-xs text-destructive">{errors.price.message}</p>}
            </div>

            <div className="space-y-2">
              <Label htmlFor="product-category">Category</Label>
              <Select
                id="product-category"
                placeholder="Select a category"
                error={!!errors.category_id}
                options={categories.map((c) => ({ value: c.id, label: c.name }))}
                {...register("category_id")}
              />
              {errors.category_id && (
                <p className="text-xs text-destructive">{errors.category_id.message}</p>
              )}
            </div>

            <div className="space-y-2">
              <Label htmlFor="product-threshold">Low stock threshold</Label>
              <Input
                id="product-threshold"
                type="number"
                min="0"
                placeholder="10"
                error={!!errors.low_stock_threshold}
                {...register("low_stock_threshold")}
              />
              {errors.low_stock_threshold && (
                <p className="text-xs text-destructive">{errors.low_stock_threshold.message}</p>
              )}
            </div>

            <div className="space-y-2 sm:col-span-2">
              <Label htmlFor="product-description">Description</Label>
              <Input
                id="product-description"
                placeholder="Optional description"
                error={!!errors.description}
                {...register("description")}
              />
              {errors.description && (
                <p className="text-xs text-destructive">{errors.description.message}</p>
              )}
            </div>
          </div>

          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onClose(false)} disabled={submitting}>
              Cancel
            </Button>
            <Button type="submit" disabled={submitting}>
              {submitting && <Loader2 className="animate-spin" />}
              {isEdit ? "Save changes" : "Create product"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}