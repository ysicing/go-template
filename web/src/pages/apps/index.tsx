import { useTranslation } from "react-i18next"
import { statsApi, userAppApi } from "@/api/services"
import OAuthClientListPage from "@/components/oauth-client/OAuthClientListPage"

export default function AppsPage() {
  const { t } = useTranslation()

  return (
    <OAuthClientListPage
      title={t("apps.title")}
      createLabel={t("apps.create")}
      newPath="/uauth/apps/new"
      listColumns={{
        name: t("apps.name"),
        clientId: t("apps.clientId"),
        scopes: t("apps.scopes"),
        users: t("apps.users"),
        logins: t("apps.logins"),
        actions: t("users.actions"),
      }}
      deleteDialog={{
        title: t("apps.delete"),
        description: t("apps.deleteConfirm"),
        successMessage: t("apps.deleted"),
      }}
      fetchList={async (page, pageSize) => {
        const res = await userAppApi.list(page, pageSize)
        return {
          clients: res.data.applications || res.data.clients || [],
          total: res.data.total || 0,
        }
      }}
      deleteItem={async (id) => {
        await userAppApi.delete(id)
      }}
      rowClickPath={(id) => `/uauth/apps/${id}/view`}
      editPath={(id) => `/uauth/apps/${id}`}
      enableStats
      fetchStats={async () => {
        const statsRes = await statsApi.user()
        return statsRes.data.app_stats || []
      }}
    />
  )
}
