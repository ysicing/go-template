import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { createUser, deleteUser, disableUser, enableUser, resetUserPassword, updateUser } from "@/subsystems/admin-users/api/users";
import { getAdminUsersErrorMessage } from "@/subsystems/admin-users/i18n";
import { UserFilters } from "@/subsystems/admin-users/components/user-filters";
import { UserFormDialog } from "@/subsystems/admin-users/components/user-form-dialog";
import { UserPagination } from "@/subsystems/admin-users/components/user-pagination";
import { UserResetPasswordDialog } from "@/subsystems/admin-users/components/user-reset-password-dialog";
import { UserTable } from "@/subsystems/admin-users/components/user-table";
import { UserViewDialog } from "@/subsystems/admin-users/components/user-view-dialog";
import { useUsers } from "@/subsystems/admin-users/hooks/use-users";
import type { AdminUser, AdminUserRole, AdminUserStatus, ResetAdminUserPasswordPayload, UserFormValues } from "@/subsystems/admin-users/types";

export function UserManagementPage() {
  const { t } = useTranslation();
  const pageSize = 10;
  const [keyword, setKeyword] = useState("");
  const [role, setRole] = useState<AdminUserRole | "">("");
  const [status, setStatus] = useState<AdminUserStatus | "">("");
  const [page, setPage] = useState(1);
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

  const query = useUsers({ keyword, role, status, page, pageSize });

  useEffect(() => {
    if (!query.data) {
      return;
    }

    const total = query.data?.total ?? 0;
    const totalPages = Math.max(1, Math.ceil(total / pageSize));

    if (page > totalPages) {
      setPage(totalPages);
    }
  }, [page, pageSize, query.data?.total]);

  function handleKeywordChange(value: string) {
    setPage(1);
    setKeyword(value);
  }

  function handleRoleChange(value: AdminUserRole | "") {
    setPage(1);
    setRole(value);
  }

  function handleStatusChange(value: AdminUserStatus | "") {
    setPage(1);
    setStatus(value);
  }

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
      setFormErrorMessage(getAdminUsersErrorMessage(t, error, t("admin_users_action_failed")));
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
      setActionErrorMessage(getAdminUsersErrorMessage(t, error, t("admin_users_action_failed")));
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
      setActionErrorMessage(getAdminUsersErrorMessage(t, error, t("admin_users_action_failed")));
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
      setResetPasswordErrorMessage(getAdminUsersErrorMessage(t, error, t("admin_users_action_failed")));
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
            onKeywordChange={handleKeywordChange}
            onRoleChange={handleRoleChange}
            onStatusChange={handleStatusChange}
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
          {query.error ? <p className="text-sm text-red-500">{getAdminUsersErrorMessage(t, query.error, t("admin_users_action_failed"))}</p> : null}
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
          <UserPagination
            currentPage={query.data?.page ?? page}
            pageSize={query.data?.page_size ?? pageSize}
            total={query.data?.total ?? 0}
            onPageChange={setPage}
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
