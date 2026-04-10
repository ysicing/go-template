import { api } from "../../../lib/api";

import type { SystemSetting } from "../types";

export async function fetchSystemSettings() {
  const response = await api.get("/system/settings");
  return response.data.data.items as SystemSetting[];
}
