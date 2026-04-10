import { FormEvent, useState } from "react";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";

import { Button } from "../components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "../components/ui/card";
import { Input } from "../components/ui/input";
import { Label } from "../components/ui/label";
import { installSystem } from "../lib/api";

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

const defaultValues: InstallFormValues = {
  server: { host: "0.0.0.0", port: 8080 },
  log: { level: "info" },
  jwt: {
    issuer: "go-template",
    access_ttl: "15m",
    refresh_ttl: "168h",
    secret: "change-me"
  },
  database: {
    driver: "sqlite",
    dsn: "file:data/app.db?_pragma=foreign_keys(1)"
  },
  cache: {
    driver: "memory",
    addr: "",
    password: "",
    db: 0
  },
  admin_username: "admin",
  admin_email: "admin@example.com",
  admin_password: "secret123"
};

export function SetupPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [values, setValues] = useState(defaultValues);
  const [error, setError] = useState("");

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    try {
      await installSystem(values);
      navigate("/login");
    } catch (submitError) {
      setError(submitError instanceof Error ? submitError.message : "初始化失败");
    }
  }

  return (
    <Card className="mx-auto w-full max-w-3xl">
      <CardHeader>
        <CardTitle>{t("setup")}</CardTitle>
        <CardDescription>类似 Gitea 的首次初始化向导</CardDescription>
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
          <Field label="Database Driver">
            <Input
              value={values.database.driver}
              onChange={(event) => setValues({ ...values, database: { ...values.database, driver: event.target.value } })}
            />
          </Field>
          <Field className="md:col-span-2" label="Database DSN">
            <Input
              value={values.database.dsn}
              onChange={(event) => setValues({ ...values, database: { ...values.database, dsn: event.target.value } })}
            />
          </Field>
          <Field label="Cache Driver">
            <Input value={values.cache.driver} onChange={(event) => setValues({ ...values, cache: { ...values.cache, driver: event.target.value } })} />
          </Field>
          <Field label="JWT Secret">
            <Input value={values.jwt.secret} onChange={(event) => setValues({ ...values, jwt: { ...values.jwt, secret: event.target.value } })} />
          </Field>
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

