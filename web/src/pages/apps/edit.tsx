import { useParams } from "react-router-dom"
import { userAppApi } from "@/api/services"
import OAuthClientEditor, { type OAuthClientFormData } from "@/components/oauth-client/OAuthClientEditor"

export default function AppEditPage() {
  const { id } = useParams()

  return (
    <OAuthClientEditor
      namespace="apps"
      id={id}
      backPath="/uauth/apps"
      onGet={async (appId) => {
        const res = await userAppApi.get(appId)
        return { client: res.data.application || res.data.client || {} }
      }}
      onCreate={async (data: OAuthClientFormData) => {
        const res = await userAppApi.create(data as unknown as Record<string, string>)
        return {
          client_secret: res.data.client_secret,
          client: res.data.application || res.data.client,
        }
      }}
      onUpdate={async (appId, data: OAuthClientFormData) => {
        await userAppApi.update(appId, data as unknown as Record<string, string>)
      }}
      showCreatedClientId
      showExistingClientId
      showEndpointInfo
    />
  )
}
