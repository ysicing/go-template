import { describe, expect, it } from "vitest";

import { getConsoleModuleEntry, getConsoleModules, getConsoleSidebarSections } from "@/app/console-navigation";

describe("console navigation", () => {
  it("returns the first visible entry for each visible module", () => {
    expect(getConsoleModuleEntry("home", { role: "user" })).toBe("/");
    expect(getConsoleModuleEntry("account", { role: "user" })).toBe("/account/profile");
    expect(getConsoleModuleEntry("admin", { role: "admin" })).toBe("/admin");
  });

  it("hides admin module for non-admin users", () => {
    expect(getConsoleModules({ role: "user" }).map((item) => item.id)).toEqual(["home", "account"]);
    expect(getConsoleSidebarSections("admin", { role: "user" })).toEqual([]);
  });
});
