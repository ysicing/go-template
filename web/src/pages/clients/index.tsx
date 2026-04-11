import { useTranslation } from "react-i18next"
import { adminClientApi } from "@/api/services"
import OAuthClientListPage from "@/components/oauth-client/OAuthClientListPage"
import { useHasPermission, adminPermissions } from "@/lib/permissions"

export default function ClientsPage() {
  const { t } = useTranslation()
  const canRead = useHasPermission(adminPermissions.clientsRead)
  const canWrite = useHasPermission(adminPermissions.clientsWrite)

  if (!canRead) {
    return <div className="text-sm text-muted-foreground">{t("common.noPermission")}</div>
  }

  return (
    <OAuthClientListPage
      title={t("clients.title")}
      createLabel={t("clients.create")}
      newPath={canWrite ? "/admin/clients/new" : undefined}
      listColumns={{
        name: t("clients.name"),
        clientId: t("clients.clientId"),
        scopes: t("clients.scopes"),
        actions: t("users.actions"),
      }}
      deleteDialog={{
        title: t("clients.delete"),
        description: t("clients.deleteConfirm"),
        successMessage: t("clients.deleted"),
      }}
      fetchList={async (page, pageSize) => {
        const res = await adminClientApi.list(page, pageSize)
        return {
          clients: res.data.clients || [],
          total: res.data.total || 0,
        }
      }}
      deleteItem={canWrite ? async (id) => {
        await adminClientApi.delete(id)
      } : undefined}
      editPath={canWrite ? ((id) => `/admin/clients/${id}`) : undefined}
    />
  )
}
