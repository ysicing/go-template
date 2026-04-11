import { Navigate, useParams } from "react-router-dom"
import { useTranslation } from "react-i18next"
import { adminClientApi } from "@/api/services"
import OAuthClientEditor, { type OAuthClientFormData } from "@/components/oauth-client/OAuthClientEditor"
import { useHasPermission, adminPermissions } from "@/lib/permissions"

export default function ClientEditPage() {
  const { id } = useParams()
  const { t } = useTranslation()
  const canWrite = useHasPermission(adminPermissions.clientsWrite)

  if (!canWrite) {
    return <Navigate to="/admin/clients" replace state={{ permissionError: t("common.noPermission") }} />
  }

  return (
    <OAuthClientEditor
      namespace="clients"
      id={id}
      backPath="/admin/clients"
      onGet={async (clientId) => {
        const res = await adminClientApi.get(clientId)
        return { client: res.data.client || {} }
      }}
      onCreate={async (data: OAuthClientFormData) => {
        const res = await adminClientApi.create(data as unknown as Record<string, string>)
        return {
          client_secret: res.data.client_secret,
          client: res.data.client,
        }
      }}
      onUpdate={async (clientId, data: OAuthClientFormData) => {
        await adminClientApi.update(clientId, data as unknown as Record<string, string>)
      }}
    />
  )
}
