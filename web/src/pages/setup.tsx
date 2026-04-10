import { FormEvent, useState } from "react";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { Database, Eye, EyeOff, HardDrive, ShieldCheck } from "lucide-react";

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

type SetupFieldErrors = Partial<Record<"admin_username" | "admin_email" | "admin_password" | "database_dsn" | "cache_addr", string>>;

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

export function SetupPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [values, setValues] = useState<InstallFormValues>(() => createDefaultValues());
  const [error, setError] = useState("");
  const [fieldErrors, setFieldErrors] = useState<SetupFieldErrors>({});
  const [showPassword, setShowPassword] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);

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
    <div className="mx-auto flex w-full max-w-4xl flex-col gap-6">
      <Card className="border-border/70 bg-card/95 shadow-sm">
        <CardHeader className="space-y-4">
          <div className="flex flex-wrap items-center gap-3">
            <Badge>{t("setup_badge")}</Badge>
            <span className="text-sm text-muted-foreground">{t("setup_summary")}</span>
          </div>
          <div className="space-y-2">
            <CardTitle className="text-2xl">{t("setup")}</CardTitle>
            <CardDescription>{t("setup_description")}</CardDescription>
          </div>
        </CardHeader>
      </Card>

      <form className="space-y-6" onSubmit={handleSubmit}>
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
                <SelectTrigger aria-label={t("database_driver")} id="setup-database-driver">
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
                <SelectTrigger aria-label={t("cache_driver")} id="setup-cache-driver">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="memory">{t("cache_driver_memory")}</SelectItem>
                  <SelectItem value="redis">{t("cache_driver_redis")}</SelectItem>
                </SelectContent>
              </Select>
            </Field>
            <div className="rounded-lg border border-dashed border-border/80 bg-muted/30 px-4 py-3 text-sm text-muted-foreground md:col-span-2">
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

        <Card className="sticky bottom-4 border-primary/20 bg-card/95 shadow-lg backdrop-blur">
          <CardContent className="flex flex-col gap-4 p-6">
            <div className="flex flex-col gap-2 md:flex-row md:items-center md:justify-between">
              <div className="space-y-1">
                <div className="text-sm font-medium">{t("setup_action_title")}</div>
                <p className="text-sm text-muted-foreground">{t("setup_action_description")}</p>
              </div>
              <Button className="w-full md:w-auto" disabled={isSubmitting} type="submit">
                {isSubmitting ? t("setup_installing") : t("install_now")}
              </Button>
            </div>
            {error ? <p className="text-sm text-red-500">{error}</p> : null}
          </CardContent>
        </Card>
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
  children: React.ReactNode;
  description: string;
  icon: React.ReactNode;
  step: string;
  title: string;
}) {
  return (
    <Card className="border-border/70 bg-card/95 shadow-sm">
      <CardHeader className="space-y-4">
        <div className="flex items-start justify-between gap-4">
          <div className="flex items-center gap-3">
            <div className="rounded-lg border border-border/70 bg-muted/40 p-2 text-muted-foreground">{icon}</div>
            <div className="space-y-1">
              <CardTitle className="text-lg">{title}</CardTitle>
              <CardDescription>{description}</CardDescription>
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

function Field({
  children,
  className,
  error,
  hint,
  htmlFor,
  label
}: {
  children: React.ReactNode;
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
        {hint ? <p className="text-sm text-muted-foreground">{hint}</p> : null}
        {error ? <p className="text-sm text-red-500">{error}</p> : null}
      </div>
    </div>
  );
}
