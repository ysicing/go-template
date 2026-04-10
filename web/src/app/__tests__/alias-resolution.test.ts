import { describe, expect, it } from "vitest";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { useIsMobile } from "@/hooks/use-mobile";
import { cn } from "@/lib/utils";

describe("path aliases", () => {
  it("resolves src aliases for shadcn structure", () => {
    expect(Badge).toBeTruthy();
    expect(Button).toBeTruthy();
    expect(typeof useIsMobile).toBe("function");
    expect(cn("a", "b")).toBe("a b");
  });
});
