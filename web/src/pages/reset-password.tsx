import { FormEvent, useMemo, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { resetPassword } from "@/lib/api";

function getErrorMessage(error: unknown, fallbackMessage: string) {
  if (
    typeof error === "object" &&
    error !== null &&
    "response" in error &&
    typeof error.response === "object" &&
    error.response !== null &&
    "data" in error.response &&
    typeof error.response.data === "object" &&
    error.response.data !== null &&
    "message" in error.response.data &&
    typeof error.response.data.message === "string"
  ) {
    return error.response.data.message;
  }

  return error instanceof Error ? error.message : fallbackMessage;
}

export function ResetPasswordPage() {
  const { t } = useTranslation();
  const [searchParams] = useSearchParams();
  const token = useMemo(() => searchParams.get("token")?.trim() ?? "", [searchParams]);
  const [newPassword, setNewPassword] = useState("");
  const [confirmNewPassword, setConfirmNewPassword] = useState("");
  const [error, setError] = useState(token ? "" : t("reset_password_missing_token"));
  const [success, setSuccess] = useState("");
  const [isSubmitting, setIsSubmitting] = useState(false);

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    setSuccess("");

    if (!token) {
      setError(t("reset_password_missing_token"));
      return;
    }

    const trimmedNewPassword = newPassword.trim();
    const trimmedConfirm = confirmNewPassword.trim();
    if (!trimmedNewPassword || !trimmedConfirm) {
      setError(t("reset_password_validation_required"));
      return;
    }
    if (trimmedNewPassword !== trimmedConfirm) {
      setError(t("reset_password_validation_mismatch"));
      return;
    }

    setIsSubmitting(true);
    try {
      const result = await resetPassword({
        confirm_new_password: trimmedConfirm,
        new_password: trimmedNewPassword,
        token
      });
      if (!result.changed) {
        setError(t("reset_password_failed"));
        return;
      }
      setNewPassword("");
      setConfirmNewPassword("");
      setSuccess(t("reset_password_success"));
    } catch (submitError) {
      setError(getErrorMessage(submitError, t("reset_password_failed")));
    } finally {
      setIsSubmitting(false);
    }
  }

  return (
    <Card className="mx-auto w-full max-w-md">
      <CardHeader>
        <CardTitle>{t("reset_password_title")}</CardTitle>
        <CardDescription>{t("reset_password_description")}</CardDescription>
      </CardHeader>
      <CardContent>
        <form className="space-y-4" onSubmit={handleSubmit}>
          <div className="space-y-2">
            <Label htmlFor="reset-password-new-password">{t("reset_password_new_password")}</Label>
            <Input
              id="reset-password-new-password"
              onChange={(event) => setNewPassword(event.target.value)}
              type="password"
              value={newPassword}
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="reset-password-confirm-new-password">{t("reset_password_confirm_new_password")}</Label>
            <Input
              id="reset-password-confirm-new-password"
              onChange={(event) => setConfirmNewPassword(event.target.value)}
              type="password"
              value={confirmNewPassword}
            />
          </div>
          {error ? <p className="text-sm text-red-500">{error}</p> : null}
          {success ? <p className="text-sm text-green-600">{success}</p> : null}
          <Button className="w-full" disabled={isSubmitting || !token} type="submit">
            {isSubmitting ? t("submitting") : t("reset_password_submit")}
          </Button>
          <div className="text-center text-sm text-muted-foreground">
            <Link className="underline underline-offset-4" to="/login">
              {t("forgot_password_back_to_login")}
            </Link>
          </div>
        </form>
      </CardContent>
    </Card>
  );
}
