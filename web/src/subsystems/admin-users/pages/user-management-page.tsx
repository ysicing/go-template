import { useState } from "react";
import { useTranslation } from "react-i18next";

import { Button } from "../../../components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "../../../components/ui/card";
import { createUser, deleteUser, disableUser, enableUser, resetUserPassword, updateUser } from "../api/users";
import { UserFilters } from "../components/user-filters";
import { UserFormDialog } from "../components/user-form-dialog";
import { UserResetPasswordDialog } from "../components/user-reset-password-dialog";
import { UserTable } from "../components/user-table";
import { UserViewDialog } from "../components/user-view-dialog";
import { useUsers } from "../hooks/use-users";
import type { AdminUser, AdminUserRole, AdminUserStatus, ResetAdminUserPasswordPayload, UserFormValues } from "../types";

function readErrorMessage(error: unknown, fallbackMessage: string) {
  if (typeof error === "object" && error !== null && "response" in error) {
    const response = (error as { response?: { data?: { message?: string } } }).response;
    if (response?.data?.message) {
      return response.data.message;
    }
  }

  return error instanceof Error ? error.message : fallbackMessage;
}

export function UserManagementPage() {
  const { t } = useTranslation();
  const [keyword, setKeyword] = useState("");
  const [role, setRole] = useState<AdminUserRole | "">("");
  const [status, setStatus] = useState<AdminUserStatus | "">("");
  const [formMode, setFormMode] = useState<"create" | "edit" | null>(null);
  const [selectedUser, setSelectedUser] = useState<AdminUser | null>(null);
  const [resetPasswordUser, setResetPasswordUser] = useState<AdminUser | null>(null);
  const [isViewOpen, setIsViewOpen] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [isResetPasswordSubmitting, setIsResetPasswordSubmitting] = useState(false);
  const [pendingUserId, setPendingUserId] = useState<number | null>(null);
  const [formErrorMessage, setFormErrorMessage] = useState<string | null>(null);
  const [resetPasswordErrorMessage, setResetPasswordErrorMessage] = useState<string | null>(null);
  const [actionErrorMessage, setActionErrorMessage] = useState<string | null>(null);
  const [actionSuccessMessage, setActionSuccessMessage] = useState<string | null>(null);

  const query = useUsers({ keyword, role, status, page: 1, pageSize: 10 });

  function openCreateDialog() {
    setSelectedUser(null);
    setFormErrorMessage(null);
    setActionSuccessMessage(null);
    setFormMode("create");
  }

  function openEditDialog(user: AdminUser) {
    setSelectedUser(user);
    setFormErrorMessage(null);
    setActionSuccessMessage(null);
    setFormMode("edit");
  }

  function openViewDialog(user: AdminUser) {
    setActionSuccessMessage(null);
    setSelectedUser(user);
    setIsViewOpen(true);
  }

  function openResetPasswordDialog(user: AdminUser) {
    setActionSuccessMessage(null);
    setResetPasswordErrorMessage(null);
    setResetPasswordUser(user);
  }

  function closeForm(open: boolean) {
    if (!open) {
      setFormMode(null);
      setFormErrorMessage(null);
    }
  }

  function closeResetPasswordDialog(open: boolean) {
    if (!open) {
      setResetPasswordUser(null);
      setResetPasswordErrorMessage(null);
    }
  }

  async function handleFormSubmit(values: UserFormValues) {
    setIsSubmitting(true);
    setFormErrorMessage(null);
    setActionSuccessMessage(null);

    try {
      if (formMode === "edit" && selectedUser) {
        await updateUser(selectedUser.id, {
          email: values.email,
          role: values.role,
          status: values.status,
          username: values.username
        });
      } else {
        await createUser(values);
      }

      setFormMode(null);
      setSelectedUser(null);
      await query.refetch();
    } catch (error) {
      setFormErrorMessage(readErrorMessage(error, t("admin_users_action_failed")));
    } finally {
      setIsSubmitting(false);
    }
  }

  async function handleToggleStatus(user: AdminUser) {
    setPendingUserId(user.id);
    setActionErrorMessage(null);
    setActionSuccessMessage(null);

    try {
      if (user.status === "active") {
        await disableUser(user.id);
      } else {
        await enableUser(user.id);
      }
      await query.refetch();
    } catch (error) {
      setActionErrorMessage(readErrorMessage(error, t("admin_users_action_failed")));
    } finally {
      setPendingUserId(null);
    }
  }

  async function handleDelete(user: AdminUser) {
    if (!window.confirm(t("admin_users_delete_confirm", { username: user.username }))) {
      return;
    }

    setPendingUserId(user.id);
    setActionErrorMessage(null);
    setActionSuccessMessage(null);

    try {
      await deleteUser(user.id);
      await query.refetch();
    } catch (error) {
      setActionErrorMessage(readErrorMessage(error, t("admin_users_action_failed")));
    } finally {
      setPendingUserId(null);
    }
  }

  async function handleResetPasswordSubmit(values: ResetAdminUserPasswordPayload) {
    if (!resetPasswordUser) {
      return;
    }

    setIsResetPasswordSubmitting(true);
    setResetPasswordErrorMessage(null);
    setActionSuccessMessage(null);

    try {
      await resetUserPassword(resetPasswordUser.id, values);
      setResetPasswordUser(null);
      setActionSuccessMessage(t("admin_users_reset_password_success", { username: resetPasswordUser.username }));
    } catch (error) {
      setResetPasswordErrorMessage(readErrorMessage(error, t("admin_users_action_failed")));
    } finally {
      setIsResetPasswordSubmitting(false);
    }
  }

  return (
    <div className="space-y-4">
      <div className="flex flex-col gap-3 xl:flex-row xl:items-center xl:justify-between">
        <div className="flex-1">
          <UserFilters
            keyword={keyword}
            role={role}
            status={status}
            onKeywordChange={setKeyword}
            onRoleChange={setRole}
            onStatusChange={setStatus}
          />
        </div>
        <Button onClick={openCreateDialog}>{t("admin_users_create_title")}</Button>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>{t("admin_users_title")}</CardTitle>
          <CardDescription>{t("admin_users_description")}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="text-sm text-muted-foreground">{t("admin_users_total", { count: query.data?.total ?? 0 })}</div>
          {query.error ? <p className="text-sm text-red-500">{readErrorMessage(query.error, t("admin_users_action_failed"))}</p> : null}
          {actionErrorMessage ? <p className="text-sm text-red-500">{actionErrorMessage}</p> : null}
          {actionSuccessMessage ? <p className="text-sm text-green-600">{actionSuccessMessage}</p> : null}
          <UserTable
            isActionPending={pendingUserId !== null || isResetPasswordSubmitting}
            isLoading={query.isLoading}
            items={query.data?.items ?? []}
            onDelete={handleDelete}
            onEdit={openEditDialog}
            onResetPassword={openResetPasswordDialog}
            onToggleStatus={handleToggleStatus}
            onView={openViewDialog}
          />
        </CardContent>
      </Card>

      <UserFormDialog
        errorMessage={formErrorMessage}
        isSubmitting={isSubmitting}
        mode={formMode ?? "create"}
        open={formMode !== null}
        user={selectedUser}
        onOpenChange={closeForm}
        onSubmit={handleFormSubmit}
      />

      <UserResetPasswordDialog
        errorMessage={resetPasswordErrorMessage}
        isSubmitting={isResetPasswordSubmitting}
        open={resetPasswordUser !== null}
        user={resetPasswordUser}
        onOpenChange={closeResetPasswordDialog}
        onSubmit={handleResetPasswordSubmit}
      />

      <UserViewDialog open={isViewOpen} user={selectedUser} onOpenChange={setIsViewOpen} />
    </div>
  );
}
