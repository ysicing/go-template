import { render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { MemoryRouter, Route, Routes } from "react-router-dom";

import { AppShell } from "@/app/layouts/app-shell";
import { AppProviders } from "@/app/providers";
import i18n from "@/lib/i18n";

describe("app shell i18n", () => {
  afterEach(async () => {
    await i18n.changeLanguage("zh-CN");
  });

  it("renders english module and sidebar labels", async () => {
    await i18n.changeLanguage("en-US");

    render(
      <AppProviders>
        <MemoryRouter initialEntries={["/admin/users"]}>
          <Routes>
            <Route
              path="*"
              element={<AppShell buildVersion="master-abc1234-20260410" user={{ email: "admin@example.com", role: "admin", username: "admin" }} />}
            >
              <Route path="*" element={<div>content</div>} />
            </Route>
          </Routes>
        </MemoryRouter>
      </AppProviders>
    );

    expect(await screen.findByRole("link", { name: "Admin" })).toHaveAttribute("aria-current", "page");
    expect(screen.getByRole("link", { name: "Home" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Account" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Overview" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Users" })).toHaveAttribute("aria-current", "page");
    expect(screen.getByRole("link", { name: "Settings" })).toBeInTheDocument();
  });
});
