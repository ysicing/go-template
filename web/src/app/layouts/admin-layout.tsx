import type { PropsWithChildren } from "react";
import { useTranslation } from "react-i18next";
import { NavLink } from "react-router-dom";

import { cn } from "@/lib/utils";
import type { NavigationItem } from "@/shared/navigation/types";

export interface AdminLayoutProps extends PropsWithChildren {
  title: string;
  description?: string;
  navigation: NavigationItem[];
}

export function AdminLayout({ title, description, navigation, children }: AdminLayoutProps) {
  const { t } = useTranslation();

  return (
    <div className="grid min-h-[calc(100vh-5rem)] gap-6 lg:grid-cols-[240px_minmax(0,1fr)]">
      <aside className="self-start rounded-xl border border-border bg-card p-4">
        <nav aria-label={t("admin_navigation_label", { section: title })} className="flex flex-col gap-2">
          {navigation.map((item) => (
            <NavLink
              end={navigation.some((candidate) => candidate.to !== item.to && candidate.to.startsWith(`${item.to}/`))}
              className={({ isActive }) =>
                cn(
                  "rounded-lg px-3 py-2 text-sm font-medium text-muted-foreground transition-colors hover:bg-muted hover:text-foreground",
                  isActive && "bg-muted text-foreground"
                )
              }
              key={item.to}
              to={item.to}
            >
              {item.label}
            </NavLink>
          ))}
        </nav>
      </aside>
      <section className="min-w-0 space-y-6">
        <header className="space-y-2">
          <h1 className="text-2xl font-semibold tracking-tight">{title}</h1>
          {description ? <p className="text-sm text-muted-foreground">{description}</p> : null}
        </header>
        {children}
      </section>
    </div>
  );
}
