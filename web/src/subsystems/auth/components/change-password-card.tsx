import { useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { changePassword } from "@/lib/api";

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

  if (error instanceof Error && error.message) {
    return error.message;
  }

  return fallbackMessage;
}

export function ChangePasswordCard() {
  const { t } = useTranslation();
  const [oldPassword, setOldPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmNewPassword, setConfirmNewPassword] = useState("");
  const [error, setError] = useState("");
  const [success, setSuccess] = useState("");
  const [isSubmitting, setIsSubmitting] = useState(false);

  function resetMessages() {
    if (error) {
      setError("");
    }

    if (success) {
      setSuccess("");
    }
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    setSuccess("");

    const trimmedOldPassword = oldPassword.trim();
    const trimmedNewPassword = newPassword.trim();
    const trimmedConfirmNewPassword = confirmNewPassword.trim();

    if (!trimmedOldPassword || !trimmedNewPassword || !trimmedConfirmNewPassword) {
      setError(t("change_password_validation_required"));
      return;
    }

    if (trimmedNewPassword !== trimmedConfirmNewPassword) {
      setError(t("change_password_validation_mismatch"));
      return;
    }

    setIsSubmitting(true);

    try {
      const result = await changePassword({
        old_password: trimmedOldPassword,
        new_password: trimmedNewPassword,
        confirm_new_password: trimmedConfirmNewPassword
      });

      if (!result.changed) {
        setError(t("change_password_failed"));
        return;
      }

      setOldPassword("");
      setNewPassword("");
      setConfirmNewPassword("");
      setSuccess(t("change_password_success"));
    } catch (submitError) {
      setError(getErrorMessage(submitError, t("change_password_failed")));
    } finally {
      setIsSubmitting(false);
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("change_password_title")}</CardTitle>
        <CardDescription>{t("change_password_description")}</CardDescription>
      </CardHeader>
      <CardContent>
        <form className="space-y-4" onSubmit={handleSubmit}>
          <div className="space-y-2">
            <Label htmlFor="old-password">{t("change_password_old_password")}</Label>
            <Input
              id="old-password"
              onChange={(event) => {
                resetMessages();
                setOldPassword(event.target.value);
              }}
              type="password"
              value={oldPassword}
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="new-password">{t("change_password_new_password")}</Label>
            <Input
              id="new-password"
              onChange={(event) => {
                resetMessages();
                setNewPassword(event.target.value);
              }}
              type="password"
              value={newPassword}
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="confirm-new-password">{t("change_password_confirm_new_password")}</Label>
            <Input
              id="confirm-new-password"
              onChange={(event) => {
                resetMessages();
                setConfirmNewPassword(event.target.value);
              }}
              type="password"
              value={confirmNewPassword}
            />
          </div>
          {error ? <p className="text-sm text-red-500">{error}</p> : null}
          {success ? <p className="text-sm text-green-600">{success}</p> : null}
          <Button className="w-full" disabled={isSubmitting} type="submit">
            {isSubmitting ? t("submitting") : t("submit")}
          </Button>
        </form>
      </CardContent>
    </Card>
  );
}
