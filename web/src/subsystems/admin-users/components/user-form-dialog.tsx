import { type FormEvent, useEffect, useState } from "react";

import { Button } from "../../../components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "../../../components/ui/card";
import { Input } from "../../../components/ui/input";
import { Label } from "../../../components/ui/label";
import { Select } from "../../../shared/ui/select";
import { Modal } from "./modal";
import type { AdminUser, UserFormValues } from "../types";

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

function validateValues(mode: "create" | "edit", values: UserFormValues) {
  if (!values.username.trim()) {
    return "请输入用户名";
  }
  if (!values.email.trim()) {
    return "请输入邮箱";
  }
  if (mode === "create" && values.password.trim().length < 8) {
    return "密码至少 8 位";
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
            关闭
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
  return (
    <>
      <Field label="用户名" name="username">
        <Input id="username" value={values.username} onChange={(event) => onChange("username", event.target.value)} />
      </Field>
      <Field label="邮箱" name="email">
        <Input id="email" type="email" value={values.email} onChange={(event) => onChange("email", event.target.value)} />
      </Field>
      {mode === "create" ? (
        <Field label="初始密码" name="password">
          <Input
            id="password"
            type="password"
            value={values.password}
            onChange={(event) => onChange("password", event.target.value)}
          />
        </Field>
      ) : null}
      <Field label="角色" name="role">
        <Select id="role" value={values.role} onChange={(event) => onChange("role", event.target.value)}>
          <option value="admin">管理员</option>
          <option value="user">普通用户</option>
        </Select>
      </Field>
      <Field label="状态" name="status">
        <Select id="status" value={values.status} onChange={(event) => onChange("status", event.target.value)}>
          <option value="active">启用</option>
          <option value="disabled">停用</option>
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
    const message = validateValues(mode, nextValues);
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
      description={mode === "create" ? "创建新的后台用户账号。" : "更新所选用户的基础资料。"}
      onClose={() => onOpenChange(false)}
      title={mode === "create" ? "新建用户" : "编辑用户"}
    >
      <form className="space-y-4" onSubmit={handleSubmit}>
        <UserFormFields mode={mode} values={values} onChange={handleChange} />
        {validationMessage ? <p className="text-sm text-red-500">{validationMessage}</p> : null}
        {errorMessage ? <p className="text-sm text-red-500">{errorMessage}</p> : null}
        <div className="flex justify-end gap-2">
          <Button size="sm" type="button" variant="outline" onClick={() => onOpenChange(false)}>
            取消
          </Button>
          <Button size="sm" disabled={isSubmitting} type="submit">
            {isSubmitting ? "提交中..." : "提交"}
          </Button>
        </div>
      </form>
    </DialogFrame>
  );
}
