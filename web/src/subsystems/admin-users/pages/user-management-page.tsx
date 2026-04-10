import { useState } from "react";

import { Button } from "../../../components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "../../../components/ui/card";
import { createUser, deleteUser, disableUser, enableUser, updateUser } from "../api/users";
import { UserFilters } from "../components/user-filters";
import { UserFormDialog } from "../components/user-form-dialog";
import { UserTable } from "../components/user-table";
import { UserViewDialog } from "../components/user-view-dialog";
import { useUsers } from "../hooks/use-users";
import type { AdminUser, AdminUserRole, AdminUserStatus, UserFormValues } from "../types";

function readErrorMessage(error: unknown) {
  if (typeof error === "object" && error !== null && "response" in error) {
    const response = (error as { response?: { data?: { message?: string } } }).response;
    if (response?.data?.message) {
      return response.data.message;
    }
  }

  return error instanceof Error ? error.message : "操作失败，请稍后重试";
}

export function UserManagementPage() {
  const [keyword, setKeyword] = useState("");
  const [role, setRole] = useState<AdminUserRole | "">("");
  const [status, setStatus] = useState<AdminUserStatus | "">("");
  const [formMode, setFormMode] = useState<"create" | "edit" | null>(null);
  const [selectedUser, setSelectedUser] = useState<AdminUser | null>(null);
  const [isViewOpen, setIsViewOpen] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [pendingUserId, setPendingUserId] = useState<number | null>(null);
  const [formErrorMessage, setFormErrorMessage] = useState<string | null>(null);
  const [actionErrorMessage, setActionErrorMessage] = useState<string | null>(null);

  const query = useUsers({ keyword, role, status, page: 1, pageSize: 10 });

  function openCreateDialog() {
    setSelectedUser(null);
    setFormErrorMessage(null);
    setFormMode("create");
  }

  function openEditDialog(user: AdminUser) {
    setSelectedUser(user);
    setFormErrorMessage(null);
    setFormMode("edit");
  }

  function openViewDialog(user: AdminUser) {
    setSelectedUser(user);
    setIsViewOpen(true);
  }

  function closeForm(open: boolean) {
    if (!open) {
      setFormMode(null);
      setFormErrorMessage(null);
    }
  }

  async function handleFormSubmit(values: UserFormValues) {
    setIsSubmitting(true);
    setFormErrorMessage(null);

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
      setFormErrorMessage(readErrorMessage(error));
    } finally {
      setIsSubmitting(false);
    }
  }

  async function handleToggleStatus(user: AdminUser) {
    setPendingUserId(user.id);
    setActionErrorMessage(null);

    try {
      if (user.status === "active") {
        await disableUser(user.id);
      } else {
        await enableUser(user.id);
      }
      await query.refetch();
    } catch (error) {
      setActionErrorMessage(readErrorMessage(error));
    } finally {
      setPendingUserId(null);
    }
  }

  async function handleDelete(user: AdminUser) {
    if (!window.confirm(`确认删除用户 ${user.username} 吗？`)) {
      return;
    }

    setPendingUserId(user.id);
    setActionErrorMessage(null);

    try {
      await deleteUser(user.id);
      await query.refetch();
    } catch (error) {
      setActionErrorMessage(readErrorMessage(error));
    } finally {
      setPendingUserId(null);
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
        <Button onClick={openCreateDialog}>新建用户</Button>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>用户列表</CardTitle>
          <CardDescription>按关键字、角色和状态筛选后台用户。</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="text-sm text-muted-foreground">共 {query.data?.total ?? 0} 位用户</div>
          {query.error ? <p className="text-sm text-red-500">{readErrorMessage(query.error)}</p> : null}
          {actionErrorMessage ? <p className="text-sm text-red-500">{actionErrorMessage}</p> : null}
          <UserTable
            isActionPending={pendingUserId !== null}
            isLoading={query.isLoading}
            items={query.data?.items ?? []}
            onDelete={handleDelete}
            onEdit={openEditDialog}
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

      <UserViewDialog open={isViewOpen} user={selectedUser} onOpenChange={setIsViewOpen} />
    </div>
  );
}
