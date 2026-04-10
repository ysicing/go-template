import { render, screen } from "@testing-library/react";
import { useEffect } from "react";
import { afterEach, describe, expect, it } from "vitest";

import { AppProviders } from "@/app/providers";
import i18n from "@/lib/i18n";
import { useAdminNavigation } from "@/app/admin-navigation";
import { useAdminRouteDefinitions } from "@/app/admin-routes";

function AdminI18nProbe() {
  const navigation = useAdminNavigation();
  const routes = useAdminRouteDefinitions();

  return (
    <>
      <div>{navigation.map((item) => item.label).join(" | ")}</div>
      <div>{routes.map((item) => item.title).join(" | ")}</div>
      <div>{routes.map((item) => item.description).join(" | ")}</div>
    </>
  );
}

function LanguageSwitcher({ language }: { language: "zh-CN" | "en-US" }) {
  useEffect(() => {
    void i18n.changeLanguage(language);
  }, [language]);

  return <AdminI18nProbe />;
}

describe("admin i18n", () => {
  afterEach(async () => {
    await i18n.changeLanguage("zh-CN");
  });

  it("returns english admin navigation and route metadata", async () => {
    render(
      <AppProviders>
        <LanguageSwitcher language="en-US" />
      </AppProviders>
    );

    expect(await screen.findByText("Overview | Users | Settings")).toBeInTheDocument();
    expect(screen.getByText("Admin Console | Users | Settings")).toBeInTheDocument();
    expect(screen.getByText("Overview of admin modules and current system status. | View, filter, and maintain user accounts. | Runtime settings and backend configuration."))
      .toBeInTheDocument();
  });
});
