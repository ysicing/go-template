import { type FormEvent, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Modal } from "./modal";
import { getAdminUserRoleLabel, getAdminUserStatusLabel } from "@/subsystems/admin-users/i18n";
import type { AdminUser, UserFormValues } from "@/subsystems/admin-users/types";

export interface UserFormDialogProps {
  open: boolean;
  mode: "create" | "edit";
  user?: AdminUser | null;
  isSubmitting?: boolean;
  errorMessage?: string | null;
  onOpenChange: (open: boolean) => void;
  onSubmit: (values: UserFormValues) => Promise<void> | void;
}

function createInitialValues(mode: "create" | "edit", user?: AdminUser | null): UserFormValues {
  return {
    username: user?.username ?? "",
    email: user?.email ?? "",
    role: user?.role ?? "user",
    status: user?.status ?? "active",
    password: mode === "create" ? "" : "********"
  };
}

function normalizeValues(mode: "create" | "edit", values: UserFormValues) {
  return {
    ...values,
    email: values.email.trim(),
    password: mode === "create" ? values.password.trim() : "",
    username: values.username.trim()
  };
}

function validateValues(mode: "create" | "edit", values: UserFormValues, t: ReturnType<typeof useTranslation>["t"]) {
  if (!values.username.trim()) {
    return t("admin_users_validation_username_required");
  }
  if (!values.email.trim()) {
    return t("admin_users_validation_email_required");
  }
  if (mode === "create" && values.password.trim().length < 8) {
    return t("admin_users_validation_password_length");
  }
  return null;
}

function DialogFrame({
  children,
  description,
  onClose,
  title
}: {
  children: React.ReactNode;
  description: string;
  onClose: () => void;
  title: string;
}) {
  const { t } = useTranslation();
  const titleId = "user-form-dialog-title";

  return (
    <Modal labelledBy={titleId} onOpenChange={() => onClose()}>
      <Card className="w-full">
        <CardHeader className="flex flex-row items-start justify-between gap-4">
          <div className="space-y-1">
            <CardTitle id={titleId}>{title}</CardTitle>
            <CardDescription>{description}</CardDescription>
          </div>
          <Button size="sm" variant="ghost" onClick={onClose}>
            {t("close")}
          </Button>
        </CardHeader>
        <CardContent>{children}</CardContent>
      </Card>
    </Modal>
  );
}

function UserFormFields({
  mode,
  values,
  onChange
}: {
  mode: "create" | "edit";
  values: UserFormValues;
  onChange: (field: keyof UserFormValues, value: string) => void;
}) {
  const { t } = useTranslation();

  return (
    <>
      <Field label={t("admin_users_field_username")} name="username">
        <Input id="username" value={values.username} onChange={(event) => onChange("username", event.target.value)} />
      </Field>
      <Field label={t("admin_users_field_email")} name="email">
        <Input id="email" type="email" value={values.email} onChange={(event) => onChange("email", event.target.value)} />
      </Field>
      {mode === "create" ? (
        <Field label={t("admin_users_field_initial_password")} name="password">
          <Input
            id="password"
            type="password"
            value={values.password}
            onChange={(event) => onChange("password", event.target.value)}
          />
        </Field>
      ) : null}
      <Field label={t("admin_users_field_role")} name="role">
        <Select value={values.role} onValueChange={(value) => onChange("role", value)}>
          <SelectTrigger aria-label={t("admin_users_field_role")} id="role">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="admin">{getAdminUserRoleLabel(t, "admin")}</SelectItem>
            <SelectItem value="user">{getAdminUserRoleLabel(t, "user")}</SelectItem>
          </SelectContent>
        </Select>
      </Field>
      <Field label={t("admin_users_field_status")} name="status">
        <Select value={values.status} onValueChange={(value) => onChange("status", value)}>
          <SelectTrigger aria-label={t("admin_users_field_status")} id="status">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="active">{getAdminUserStatusLabel(t, "active")}</SelectItem>
            <SelectItem value="disabled">{getAdminUserStatusLabel(t, "disabled")}</SelectItem>
          </SelectContent>
        </Select>
      </Field>
    </>
  );
}

function Field({
  children,
  label,
  name
}: {
  children: React.ReactNode;
  label: string;
  name: string;
}) {
  return (
    <div className="space-y-2">
      <Label htmlFor={name}>{label}</Label>
      {children}
    </div>
  );
}

export function UserFormDialog({
  open,
  mode,
  user,
  isSubmitting = false,
  errorMessage = null,
  onOpenChange,
  onSubmit
}: UserFormDialogProps) {
  const { t } = useTranslation();
  const [values, setValues] = useState<UserFormValues>(() => createInitialValues(mode, user));
  const [validationMessage, setValidationMessage] = useState<string | null>(null);

  useEffect(() => {
    setValues(createInitialValues(mode, user));
    setValidationMessage(null);
  }, [mode, user, open]);

  if (!open) {
    return null;
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();

    const nextValues = normalizeValues(mode, values);
    const message = validateValues(mode, nextValues, t);
    if (message) {
      setValidationMessage(message);
      return;
    }

    setValidationMessage(null);
    await onSubmit(nextValues);
  }

  function handleChange(field: keyof UserFormValues, value: string) {
    setValues((current) => ({ ...current, [field]: value }));
  }

  return (
    <DialogFrame
      description={mode === "create" ? t("admin_users_create_description") : t("admin_users_edit_description")}
      onClose={() => onOpenChange(false)}
      title={mode === "create" ? t("admin_users_create_title") : t("admin_users_edit_title")}
    >
      <form className="space-y-4" onSubmit={handleSubmit}>
        <UserFormFields mode={mode} values={values} onChange={handleChange} />
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
    </DialogFrame>
  );
}
