import { describe, expect, it } from "vitest"

import { adminPermissions } from "@/lib/permissions"
import { getConsoleModules } from "@/lib/navigation"

describe("console navigation", () => {
  it("hides removed modules and admin tools in the template", () => {
    const modules = getConsoleModules({
      id: "user-1",
      username: "alice",
      email: "alice@example.com",
      is_admin: false,
      permissions: [adminPermissions.settingsRead, adminPermissions.pointsRead],
      email_verified: true,
    })

    const serialized = JSON.stringify(modules)

    expect(serialized).not.toContain("organizations")
    expect(serialized).not.toContain("workspace-plans")
    expect(serialized).not.toContain("service-accounts")
    expect(serialized).not.toContain("webhooks")
    expect(serialized).not.toContain("monitoring")
    expect(serialized).not.toContain("quotes")
    expect(serialized).not.toContain("telegram-moderation")
  })
})
