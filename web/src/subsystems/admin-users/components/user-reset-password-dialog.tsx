import { type FormEvent, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Modal } from "./modal";
import type { AdminUser, ResetAdminUserPasswordPayload } from "@/subsystems/admin-users/types";

export interface UserResetPasswordDialogProps {
  open: boolean;
  user?: AdminUser | null;
  isSubmitting?: boolean;
  errorMessage?: string | null;
  onOpenChange: (open: boolean) => void;
  onSubmit: (values: ResetAdminUserPasswordPayload) => Promise<void> | void;
}

function createInitialValues(): ResetAdminUserPasswordPayload {
  return {
    confirm_new_password: "",
    new_password: ""
  };
}

function validateValues(values: ResetAdminUserPasswordPayload, t: ReturnType<typeof useTranslation>["t"]) {
  if (values.new_password.trim().length < 8) {
    return t("admin_users_validation_password_length");
  }
  if (values.new_password.trim() !== values.confirm_new_password.trim()) {
    return t("admin_users_validation_password_mismatch");
  }
  return null;
}

export function UserResetPasswordDialog({
  open,
  user,
  isSubmitting = false,
  errorMessage = null,
  onOpenChange,
  onSubmit
}: UserResetPasswordDialogProps) {
  const { t } = useTranslation();
  const [values, setValues] = useState<ResetAdminUserPasswordPayload>(createInitialValues);
  const [validationMessage, setValidationMessage] = useState<string | null>(null);
  const titleId = "user-reset-password-dialog-title";

  useEffect(() => {
    setValues(createInitialValues());
    setValidationMessage(null);
  }, [open, user]);

  if (!open || !user) {
    return null;
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();

    const nextValues = {
      confirm_new_password: values.confirm_new_password.trim(),
      new_password: values.new_password.trim()
    };
    const message = validateValues(nextValues, t);
    if (message) {
      setValidationMessage(message);
      return;
    }

    setValidationMessage(null);
    await onSubmit(nextValues);
  }

  function handleChange(field: keyof ResetAdminUserPasswordPayload, value: string) {
    setValues((current) => ({ ...current, [field]: value }));
  }

  return (
    <Modal labelledBy={titleId} onOpenChange={() => onOpenChange(false)}>
      <Card className="w-full max-w-md">
        <CardHeader className="flex flex-row items-start justify-between gap-4">
          <div className="space-y-1">
            <CardTitle id={titleId}>{t("admin_users_reset_password_title")}</CardTitle>
            <CardDescription>{t("admin_users_reset_password_description", { username: user.username })}</CardDescription>
          </div>
          <Button size="sm" variant="ghost" onClick={() => onOpenChange(false)}>
            {t("close")}
          </Button>
        </CardHeader>
        <CardContent>
          <form className="space-y-4" onSubmit={handleSubmit}>
            <div className="space-y-2">
              <Label htmlFor="reset-password-new">{t("admin_users_reset_password_new_password")}</Label>
              <Input
                id="reset-password-new"
                type="password"
                value={values.new_password}
                onChange={(event) => handleChange("new_password", event.target.value)}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="reset-password-confirm">{t("admin_users_reset_password_confirm_new_password")}</Label>
              <Input
                id="reset-password-confirm"
                type="password"
                value={values.confirm_new_password}
                onChange={(event) => handleChange("confirm_new_password", event.target.value)}
              />
            </div>
            {validationMessage ? <p className="text-sm text-red-500">{validationMessage}</p> : null}
            {errorMessage ? <p className="text-sm text-red-500">{errorMessage}</p> : null}
            <div className="flex justify-end gap-2">
              <Button size="sm" type="button" variant="outline" onClick={() => onOpenChange(false)}>
                {t("cancel")}
              </Button>
              <Button size="sm" disabled={isSubmitting} type="submit">
                {isSubmitting ? t("submitting") : t("submit")}
              </Button>
            </div>
          </form>
        </CardContent>
      </Card>
    </Modal>
  );
}
