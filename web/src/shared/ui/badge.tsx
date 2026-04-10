import * as React from "react";

import { cn } from "../../lib/utils";

export function Badge({ className, ...props }: React.HTMLAttributes<HTMLSpanElement>) {
  return (
    <span
      className={cn("inline-flex items-center rounded-full px-2.5 py-1 text-xs font-medium transition-colors", className)}
      {...props}
    />
  );
}

export function StatusBadge({ status }: { status: "active" | "disabled" }) {
  return (
    <Badge
      className={
        status === "active"
          ? "bg-accent/15 text-foreground ring-1 ring-inset ring-accent/20"
          : "bg-muted text-muted-foreground ring-1 ring-inset ring-border"
      }
    >
      {status}
    </Badge>
  );
}
