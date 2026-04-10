import { FormEvent, useState } from "react";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
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

export function SetupPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [values, setValues] = useState<InstallFormValues>(() => createDefaultValues());
  const [error, setError] = useState("");

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    try {
      await installSystem(values);
      navigate("/login");
    } catch (submitError) {
      setError(submitError instanceof Error ? submitError.message : t("setup_failed"));
    }
  }

  return (
    <Card className="mx-auto w-full max-w-3xl">
      <CardHeader>
        <CardTitle>{t("setup")}</CardTitle>
      </CardHeader>
      <CardContent>
        <form className="grid gap-4 md:grid-cols-2" onSubmit={handleSubmit}>
          <Field label={t("admin_username")}>
            <Input value={values.admin_username} onChange={(event) => setValues({ ...values, admin_username: event.target.value })} />
          </Field>
          <Field label={t("admin_email")}>
            <Input value={values.admin_email} onChange={(event) => setValues({ ...values, admin_email: event.target.value })} />
          </Field>
          <Field label={t("admin_password")}>
            <Input
              type="password"
              value={values.admin_password}
              onChange={(event) => setValues({ ...values, admin_password: event.target.value })}
            />
          </Field>
          <Field label={t("database_driver")}>
            <Select
              value={values.database.driver}
              onValueChange={(value) =>
                setValues({
                  ...values,
                  database: {
                    ...values.database,
                    driver: value,
                    dsn: getDatabaseDSN(value)
                  }
                })
              }
            >
              <SelectTrigger aria-label={t("database_driver")}>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="sqlite">{t("database_driver_sqlite")}</SelectItem>
                <SelectItem value="postgres">{t("database_driver_postgres")}</SelectItem>
                <SelectItem value="mysql">{t("database_driver_mysql")}</SelectItem>
              </SelectContent>
            </Select>
          </Field>
          <Field className="md:col-span-2" label={t("database_dsn")}>
            <Input
              placeholder={getDatabaseDSN(values.database.driver)}
              value={values.database.dsn}
              onChange={(event) => setValues({ ...values, database: { ...values.database, dsn: event.target.value } })}
            />
          </Field>
          <Field label={t("cache_driver")}>
            <Select
              value={values.cache.driver}
              onValueChange={(value) =>
                setValues({
                  ...values,
                  cache: {
                    driver: value,
                    ...getCacheDefaults(value)
                  }
                })
              }
            >
              <SelectTrigger aria-label={t("cache_driver")}>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="memory">{t("cache_driver_memory")}</SelectItem>
                <SelectItem value="redis">{t("cache_driver_redis")}</SelectItem>
              </SelectContent>
            </Select>
          </Field>
          {values.cache.driver === "redis" ? (
            <>
              <Field label={t("cache_addr")}>
                <Input
                  value={values.cache.addr}
                  onChange={(event) => setValues({ ...values, cache: { ...values.cache, addr: event.target.value } })}
                />
              </Field>
              <Field label={t("cache_password")}>
                <Input
                  type="password"
                  value={values.cache.password}
                  onChange={(event) => setValues({ ...values, cache: { ...values.cache, password: event.target.value } })}
                />
              </Field>
              <Field label={t("cache_db")}>
                <Input
                  type="number"
                  value={values.cache.db}
                  onChange={(event) =>
                    setValues({ ...values, cache: { ...values.cache, db: Number.parseInt(event.target.value || "0", 10) || 0 } })
                  }
                />
              </Field>
            </>
          ) : null}
          {error ? <p className="text-sm text-red-500 md:col-span-2">{error}</p> : null}
          <Button className="md:col-span-2" type="submit">
            {t("install_now")}
          </Button>
        </form>
      </CardContent>
    </Card>
  );
}

function Field({
  children,
  className,
  label
}: {
  children: React.ReactNode;
  className?: string;
  label: string;
}) {
  return (
    <div className={className}>
      <div className="space-y-2">
        <Label>{label}</Label>
        {children}
      </div>
    </div>
  );
}
