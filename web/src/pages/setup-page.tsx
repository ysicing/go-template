import { useState, type FormEvent, type ReactNode } from "react";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { CheckCircle2, Database, Eye, EyeOff, HardDrive, ShieldCheck, Sparkles } from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Separator } from "@/components/ui/separator";
import { installSystem } from "@/lib/api";

export type InstallFormValues = {
  server: {
    host: string;
    port: number;
  };
  log: {
    level: string;
  };
  jwt: {
    issuer: string;
    access_ttl: string;
    refresh_ttl: string;
    secret: string;
  };
  database: {
    driver: string;
    dsn: string;
  };
  cache: {
    driver: string;
    addr: string;
    password: string;
    db: number;
  };
  admin_username: string;
  admin_email: string;
  admin_password: string;
};

type SetupFieldErrors = Partial<Record<"admin_username" | "admin_email" | "admin_password" | "database_dsn" | "cache_addr", string>>;

function generateJWTSecret() {
  const bytes = new Uint8Array(24);
  globalThis.crypto.getRandomValues(bytes);
  return Array.from(bytes, (value) => value.toString(16).padStart(2, "0")).join("");
}

function getDatabaseDSN(driver: string) {
  switch (driver) {
    case "postgres":
      return "postgres://postgres:postgres@127.0.0.1:5432/app?sslmode=disable";
    case "mysql":
      return "root:password@tcp(127.0.0.1:3306)/app?charset=utf8mb4&parseTime=True&loc=Local";
    default:
      return "file:data/app.db?_pragma=foreign_keys(1)";
  }
}

function getCacheDefaults(driver: string) {
  if (driver === "redis") {
    return {
      addr: "127.0.0.1:6379",
      db: 0,
      password: ""
    };
  }

  return {
    addr: "",
    db: 0,
    password: ""
  };
}

function createDefaultValues(): InstallFormValues {
  return {
    server: { host: "0.0.0.0", port: 3206 },
    log: { level: "info" },
    jwt: {
      issuer: "go-template",
      access_ttl: "15m",
      refresh_ttl: "168h",
      secret: generateJWTSecret()
    },
    database: {
      driver: "sqlite",
      dsn: getDatabaseDSN("sqlite")
    },
    cache: {
      driver: "memory",
      ...getCacheDefaults("memory")
    },
    admin_username: "admin",
    admin_email: "admin@example.com",
    admin_password: "secret123"
  };
}

function getErrorMessage(error: unknown, fallbackMessage: string) {
  if (error instanceof Error) {
    return mapSetupErrorMessage(error.message, fallbackMessage);
  }

  return error instanceof Error ? error.message : fallbackMessage;
}

function mapSetupErrorMessage(message: string, fallbackMessage: string) {
  switch (message) {
    case "admin username is required":
      return "请输入管理员用户名";
    case "admin email is required":
      return "请输入管理员邮箱";
    case "admin password must be at least 8 characters":
      return "管理员密码至少需要 8 位";
    case "database dsn is required":
      return "请输入数据库 DSN";
    default:
      return message || fallbackMessage;
  }
}

function validateSetup(values: InstallFormValues, t: ReturnType<typeof useTranslation>["t"]) {
  const fieldErrors: SetupFieldErrors = {};

  if (!values.admin_username.trim()) {
    fieldErrors.admin_username = t("setup_validation_admin_username_required");
  }
  if (!values.admin_email.trim()) {
    fieldErrors.admin_email = t("setup_validation_admin_email_required");
  }
  if (values.admin_password.trim().length < 8) {
    fieldErrors.admin_password = t("setup_validation_admin_password_length");
  }
  if (!values.database.dsn.trim()) {
    fieldErrors.database_dsn = t("setup_validation_database_dsn_required");
  }
  if (values.cache.driver === "redis" && !values.cache.addr.trim()) {
    fieldErrors.cache_addr = t("setup_validation_cache_addr_required");
  }

  return fieldErrors;
}

function getDatabaseDriverLabel(driver: string, t: ReturnType<typeof useTranslation>["t"]) {
  switch (driver) {
    case "postgres":
      return t("database_driver_postgres");
    case "mysql":
      return t("database_driver_mysql");
    default:
      return t("database_driver_sqlite");
  }
}

function getCacheDriverLabel(driver: string, t: ReturnType<typeof useTranslation>["t"]) {
  return driver === "redis" ? t("cache_driver_redis") : t("cache_driver_memory");
}

export function SetupPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [values, setValues] = useState<InstallFormValues>(() => createDefaultValues());
  const [error, setError] = useState("");
  const [fieldErrors, setFieldErrors] = useState<SetupFieldErrors>({});
  const [showPassword, setShowPassword] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);

  const setupSteps = [
    {
      step: "01",
      title: t("setup_database_title"),
      description: t("setup_database_description"),
      icon: <Database className="h-5 w-5" />,
      complete: Boolean(values.database.driver && values.database.dsn.trim())
    },
    {
      step: "02",
      title: t("setup_cache_title"),
      description: t("setup_cache_description"),
      icon: <HardDrive className="h-5 w-5" />,
      complete: values.cache.driver === "memory" || Boolean(values.cache.addr.trim())
    },
    {
      step: "03",
      title: t("setup_admin_title"),
      description: t("setup_admin_description"),
      icon: <ShieldCheck className="h-5 w-5" />,
      complete: Boolean(values.admin_username.trim() && values.admin_email.trim() && values.admin_password.trim().length >= 8)
    }
  ];

  function clearFieldError(field: keyof SetupFieldErrors) {
    setFieldErrors((current) => {
      if (!current[field]) {
        return current;
      }

      const next = { ...current };
      delete next[field];
      return next;
    });
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    const nextFieldErrors = validateSetup(values, t);
    setFieldErrors(nextFieldErrors);
    const firstError = Object.values(nextFieldErrors)[0];
    if (firstError) {
      setError(firstError);
      return;
    }

    setIsSubmitting(true);

    try {
      await installSystem(values);
      navigate("/login");
    } catch (submitError) {
      setError(getErrorMessage(submitError, t("setup_failed")));
    } finally {
      setIsSubmitting(false);
    }
  }

  return (
    <div className="mx-auto flex w-full max-w-7xl flex-col gap-6">
      <section className="overflow-hidden rounded-[32px] border border-slate-800 bg-gradient-to-br from-slate-950 via-slate-900 to-slate-800 text-white shadow-2xl shadow-slate-950/20">
        <div className="grid gap-6 px-6 py-8 lg:grid-cols-[1.15fr_0.85fr] lg:px-8">
          <div className="space-y-5">
            <div className="flex flex-wrap items-center gap-3">
              <Badge className="border-white/15 bg-white/10 text-white hover:bg-white/10">{t("setup_badge")}</Badge>
              <span className="text-sm text-slate-300">{t("setup_summary")}</span>
            </div>
            <div className="space-y-3">
              <div className="inline-flex items-center gap-2 rounded-full border border-white/10 bg-white/5 px-3 py-1 text-sm text-slate-200">
                <Sparkles className="h-4 w-4 text-sky-300" />
                <span>{t("setup_hero_chip")}</span>
              </div>
              <h1 className="max-w-3xl text-3xl font-semibold tracking-tight sm:text-4xl">{t("setup")}</h1>
              <p className="max-w-2xl text-sm leading-6 text-slate-300 sm:text-base">{t("setup_description")}</p>
            </div>
            <div className="grid gap-3 sm:grid-cols-3">
              {setupSteps.map((step) => (
                <div key={step.step} className="rounded-2xl border border-white/10 bg-white/8 p-4 backdrop-blur">
                  <div className="flex items-start justify-between gap-3">
                    <div className="space-y-1">
                      <p className="text-xs uppercase tracking-[0.22em] text-slate-400">{step.step}</p>
                      <p className="text-sm font-medium text-white">{step.title}</p>
                    </div>
                    <div className="rounded-xl bg-white/10 p-2 text-sky-200">{step.icon}</div>
                  </div>
                  <p className="mt-3 text-sm leading-6 text-slate-300">{step.description}</p>
                </div>
              ))}
            </div>
          </div>

          <div className="grid gap-3 sm:grid-cols-3 lg:grid-cols-1">
            {setupSteps.map((step) => (
              <div key={step.title} className="rounded-2xl border border-white/10 bg-white/8 p-4 backdrop-blur">
                <div className="flex items-start justify-between gap-3">
                  <div className="space-y-1">
                    <p className="text-sm text-slate-300">{step.title}</p>
                    <p className="text-xs leading-5 text-slate-400">{step.description}</p>
                  </div>
                  <CheckCircle2 className={step.complete ? "h-5 w-5 text-emerald-300" : "h-5 w-5 text-slate-500"} />
                </div>
              </div>
            ))}
          </div>
        </div>
      </section>

      <form className="grid gap-6 xl:grid-cols-[1.1fr_0.9fr]" onSubmit={handleSubmit}>
        <div className="space-y-6">
          <SectionCard
            description={t("setup_database_description")}
            icon={<Database className="h-5 w-5" />}
            step="01"
            title={t("setup_database_title")}
          >
            <div className="grid gap-4 md:grid-cols-2">
              <Field hint={t("setup_database_driver_hint")} label={t("database_driver")}>
                <Select
                  value={values.database.driver}
                  onValueChange={(value) =>
                    setValues((current) => ({
                      ...current,
                      database: {
                        ...current.database,
                        driver: value,
                        dsn: getDatabaseDSN(value)
                      }
                    }))
                  }
                >
                  <SelectTrigger aria-label={t("database_driver")} id="setup-database-driver" className="bg-background">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="sqlite">{t("database_driver_sqlite")}</SelectItem>
                    <SelectItem value="postgres">{t("database_driver_postgres")}</SelectItem>
                    <SelectItem value="mysql">{t("database_driver_mysql")}</SelectItem>
                  </SelectContent>
                </Select>
              </Field>
              <Field
                className="md:col-span-2"
                error={fieldErrors.database_dsn}
                hint={t("setup_database_dsn_hint")}
                htmlFor="setup-database-dsn"
                label={t("database_dsn")}
              >
                <Input
                  aria-invalid={Boolean(fieldErrors.database_dsn)}
                  id="setup-database-dsn"
                  placeholder={getDatabaseDSN(values.database.driver)}
                  value={values.database.dsn}
                  onChange={(event) => {
                    clearFieldError("database_dsn");
                    setValues((current) => ({
                      ...current,
                      database: {
                        ...current.database,
                        dsn: event.target.value
                      }
                    }));
                  }}
                />
              </Field>
            </div>
          </SectionCard>

          <SectionCard
            description={t("setup_cache_description")}
            icon={<HardDrive className="h-5 w-5" />}
            step="02"
            title={t("setup_cache_title")}
          >
            <div className="grid gap-4 md:grid-cols-2">
              <Field hint={t("setup_cache_driver_hint")} label={t("cache_driver")}>
                <Select
                  value={values.cache.driver}
                  onValueChange={(value) =>
                    setValues((current) => ({
                      ...current,
                      cache: {
                        driver: value,
                        ...getCacheDefaults(value)
                      }
                    }))
                  }
                >
                  <SelectTrigger aria-label={t("cache_driver")} id="setup-cache-driver" className="bg-background">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="memory">{t("cache_driver_memory")}</SelectItem>
                    <SelectItem value="redis">{t("cache_driver_redis")}</SelectItem>
                  </SelectContent>
                </Select>
              </Field>
              <div className="rounded-2xl border border-dashed border-border/80 bg-muted/30 px-4 py-3 text-sm text-muted-foreground md:col-span-2">
                {values.cache.driver === "redis" ? t("setup_cache_redis_hint") : t("setup_cache_memory_hint")}
              </div>
              {values.cache.driver === "redis" ? (
                <>
                  <Field error={fieldErrors.cache_addr} htmlFor="setup-cache-addr" label={t("cache_addr")}>
                    <Input
                      aria-invalid={Boolean(fieldErrors.cache_addr)}
                      id="setup-cache-addr"
                      value={values.cache.addr}
                      onChange={(event) => {
                        clearFieldError("cache_addr");
                        setValues((current) => ({
                          ...current,
                          cache: {
                            ...current.cache,
                            addr: event.target.value
                          }
                        }));
                      }}
                    />
                  </Field>
                  <Field htmlFor="setup-cache-password" label={t("cache_password")}>
                    <Input
                      id="setup-cache-password"
                      type="password"
                      value={values.cache.password}
                      onChange={(event) =>
                        setValues((current) => ({
                          ...current,
                          cache: {
                            ...current.cache,
                            password: event.target.value
                          }
                        }))
                      }
                    />
                  </Field>
                  <Field htmlFor="setup-cache-db" label={t("cache_db")}>
                    <Input
                      id="setup-cache-db"
                      type="number"
                      value={values.cache.db}
                      onChange={(event) =>
                        setValues((current) => ({
                          ...current,
                          cache: {
                            ...current.cache,
                            db: Number.parseInt(event.target.value || "0", 10) || 0
                          }
                        }))
                      }
                    />
                  </Field>
                </>
              ) : null}
            </div>
          </SectionCard>

          <SectionCard
            description={t("setup_admin_description")}
            icon={<ShieldCheck className="h-5 w-5" />}
            step="03"
            title={t("setup_admin_title")}
          >
            <div className="grid gap-4 md:grid-cols-2">
              <Field error={fieldErrors.admin_username} htmlFor="setup-admin-username" label={t("admin_username")}>
                <Input
                  aria-invalid={Boolean(fieldErrors.admin_username)}
                  id="setup-admin-username"
                  value={values.admin_username}
                  onChange={(event) => {
                    clearFieldError("admin_username");
                    setValues((current) => ({ ...current, admin_username: event.target.value }));
                  }}
                />
              </Field>
              <Field error={fieldErrors.admin_email} htmlFor="setup-admin-email" label={t("admin_email")}>
                <Input
                  aria-invalid={Boolean(fieldErrors.admin_email)}
                  id="setup-admin-email"
                  type="email"
                  value={values.admin_email}
                  onChange={(event) => {
                    clearFieldError("admin_email");
                    setValues((current) => ({ ...current, admin_email: event.target.value }));
                  }}
                />
              </Field>
              <Field
                className="md:col-span-2"
                error={fieldErrors.admin_password}
                hint={t("setup_admin_password_hint")}
                htmlFor="setup-admin-password"
                label={t("admin_password")}
              >
                <div className="relative">
                  <Input
                    aria-invalid={Boolean(fieldErrors.admin_password)}
                    className="pr-12"
                    id="setup-admin-password"
                    type={showPassword ? "text" : "password"}
                    value={values.admin_password}
                    onChange={(event) => {
                      clearFieldError("admin_password");
                      setValues((current) => ({ ...current, admin_password: event.target.value }));
                    }}
                  />
                  <Button
                    aria-label={showPassword ? t("setup_hide_password") : t("setup_show_password")}
                    className="absolute right-1 top-1 h-8 w-8"
                    size="icon"
                    type="button"
                    variant="ghost"
                    onClick={() => setShowPassword((current) => !current)}
                  >
                    {showPassword ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                  </Button>
                </div>
              </Field>
            </div>
          </SectionCard>
        </div>

        <div className="space-y-6">
          <Card className="top-6 rounded-3xl border-border/80 bg-card/95 shadow-sm xl:sticky">
            <CardHeader className="space-y-3">
              <div className="inline-flex w-fit items-center gap-2 rounded-full border bg-muted/40 px-3 py-1 text-xs text-muted-foreground">
                <Sparkles className="h-3.5 w-3.5" />
                <span>{t("setup_overview_badge")}</span>
              </div>
              <div className="space-y-1">
                <CardTitle>{t("setup_overview_title")}</CardTitle>
                <CardDescription>{t("setup_overview_description")}</CardDescription>
              </div>
            </CardHeader>
            <CardContent className="space-y-5">
              <SummaryItem
                label={t("setup_overview_database")}
                value={getDatabaseDriverLabel(values.database.driver, t)}
                description={values.database.dsn}
              />
              <SummaryItem
                label={t("setup_overview_cache")}
                value={getCacheDriverLabel(values.cache.driver, t)}
                description={values.cache.driver === "redis" ? values.cache.addr || t("setup_overview_pending") : t("setup_cache_memory_hint")}
              />
              <SummaryItem
                label={t("setup_overview_admin")}
                value={values.admin_username || t("setup_overview_pending")}
                description={values.admin_email || t("setup_overview_pending")}
              />

              <Separator />

              <div className="rounded-2xl border bg-muted/20 p-4">
                <div className="space-y-1">
                  <p className="text-sm font-medium">{t("setup_action_title")}</p>
                  <p className="text-sm leading-6 text-muted-foreground">{t("setup_action_description")}</p>
                </div>
              </div>

              {error ? <p className="rounded-2xl border border-red-500/20 bg-red-500/5 px-4 py-3 text-sm text-red-500">{error}</p> : null}

              <Button className="h-11 w-full rounded-xl" disabled={isSubmitting} type="submit">
                {isSubmitting ? t("setup_installing") : t("install_now")}
              </Button>
            </CardContent>
          </Card>
        </div>
      </form>
    </div>
  );
}

function SectionCard({
  children,
  description,
  icon,
  step,
  title
}: {
  children: ReactNode;
  description: string;
  icon: ReactNode;
  step: string;
  title: string;
}) {
  return (
    <Card className="rounded-3xl border-border/80 bg-card/95 shadow-sm">
      <CardHeader className="space-y-4">
        <div className="flex items-start justify-between gap-4">
          <div className="flex items-center gap-3">
            <div className="rounded-2xl border border-border/70 bg-muted/40 p-2.5 text-muted-foreground">{icon}</div>
            <div className="space-y-1">
              <CardTitle className="text-lg">{title}</CardTitle>
              <CardDescription className="leading-6">{description}</CardDescription>
            </div>
          </div>
          <Badge variant="outline">{step}</Badge>
        </div>
        <Separator />
      </CardHeader>
      <CardContent>{children}</CardContent>
    </Card>
  );
}

function SummaryItem({
  label,
  value,
  description
}: {
  label: string;
  value: string;
  description: string;
}) {
  return (
    <div className="rounded-2xl border bg-background p-4">
      <p className="text-sm text-muted-foreground">{label}</p>
      <p className="mt-1 font-medium">{value}</p>
      <p className="mt-2 break-all text-sm leading-6 text-muted-foreground">{description}</p>
    </div>
  );
}

function Field({
  children,
  className,
  error,
  hint,
  htmlFor,
  label
}: {
  children: ReactNode;
  className?: string;
  error?: string;
  hint?: string;
  htmlFor?: string;
  label: string;
}) {
  return (
    <div className={className}>
      <div className="space-y-2">
        <Label htmlFor={htmlFor}>{label}</Label>
        {children}
        {hint ? <p className="text-sm leading-6 text-muted-foreground">{hint}</p> : null}
        {error ? <p className="text-sm text-red-500">{error}</p> : null}
      </div>
    </div>
  );
}
