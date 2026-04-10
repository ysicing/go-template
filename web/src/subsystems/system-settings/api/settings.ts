import { api, fetchMailSettings as fetchMailSettingsFromAPI, updateMailSettings as updateMailSettingsFromAPI } from "@/lib/api";

import type { RuntimeMailSettings, SystemSetting } from "@/subsystems/system-settings/types";

export async function fetchSystemSettings() {
  const response = await api.get("/system/settings");
  return response.data.data.items as SystemSetting[];
}

export async function fetchMailSettings() {
  const data = await fetchMailSettingsFromAPI();
  return {
    ...data,
    password: ""
  } as RuntimeMailSettings;
}

export async function updateMailSettings(payload: RuntimeMailSettings) {
  const data = await updateMailSettingsFromAPI(payload);
  return {
    ...data,
    password: ""
  } as RuntimeMailSettings;
}
