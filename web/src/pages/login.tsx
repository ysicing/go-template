import { FormEvent, useState } from "react";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";

import { Button } from "../components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "../components/ui/card";
import { Input } from "../components/ui/input";
import { Label } from "../components/ui/label";
import { login } from "../lib/api";

export function LoginPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [identifier, setIdentifier] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    try {
      const user = await login({ identifier, password });
      navigate(user.role === "admin" ? "/admin" : "/");
    } catch (submitError) {
      setError(submitError instanceof Error ? submitError.message : "登录失败");
    }
  }

  return (
    <Card className="mx-auto w-full max-w-md">
      <CardHeader>
        <CardTitle>{t("login")}</CardTitle>
        <CardDescription>{t("login_description")}</CardDescription>
      </CardHeader>
      <CardContent>
        <form className="space-y-4" onSubmit={handleSubmit}>
          <div className="space-y-2">
            <Label htmlFor="identifier">{t("login_identifier")}</Label>
            <Input id="identifier" onChange={(event) => setIdentifier(event.target.value)} value={identifier} />
          </div>
          <div className="space-y-2">
            <Label htmlFor="password">{t("password")}</Label>
            <Input id="password" onChange={(event) => setPassword(event.target.value)} type="password" value={password} />
          </div>
          {error ? <p className="text-sm text-red-500">{error}</p> : null}
          <Button className="w-full" type="submit">
            {t("login")}
          </Button>
        </form>
      </CardContent>
    </Card>
  );
}
