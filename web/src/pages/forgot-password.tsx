import { FormEvent, useState } from "react";
import { Link } from "react-router-dom";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { requestPasswordReset } from "@/lib/api";

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

export function ForgotPasswordPage() {
  const { t } = useTranslation();
  const [email, setEmail] = useState("");
  const [error, setError] = useState("");
  const [success, setSuccess] = useState("");
  const [isSubmitting, setIsSubmitting] = useState(false);

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    setSuccess("");

    const normalizedEmail = email.trim();
    if (!normalizedEmail) {
      setError(t("forgot_password_validation_email_required"));
      return;
    }

    setIsSubmitting(true);
    try {
      const result = await requestPasswordReset({ email: normalizedEmail });
      if (!result.sent) {
        setError(t("forgot_password_failed"));
        return;
      }
      setSuccess(t("forgot_password_success"));
    } catch (submitError) {
      setError(getErrorMessage(submitError, t("forgot_password_failed")));
    } finally {
      setIsSubmitting(false);
    }
  }

  return (
    <Card className="mx-auto w-full max-w-md">
      <CardHeader>
        <CardTitle>{t("forgot_password_title")}</CardTitle>
        <CardDescription>{t("forgot_password_description")}</CardDescription>
      </CardHeader>
      <CardContent>
        <form className="space-y-4" onSubmit={handleSubmit}>
          <div className="space-y-2">
            <Label htmlFor="forgot-password-email">{t("forgot_password_email")}</Label>
            <Input
              id="forgot-password-email"
              onChange={(event) => setEmail(event.target.value)}
              type="email"
              value={email}
            />
          </div>
          {error ? <p className="text-sm text-red-500">{error}</p> : null}
          {success ? <p className="text-sm text-green-600">{success}</p> : null}
          <Button className="w-full" disabled={isSubmitting} type="submit">
            {isSubmitting ? t("submitting") : t("forgot_password_submit")}
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
