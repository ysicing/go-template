export type SystemSetting = {
  id: number;
  key: string;
  value: string;
  group: string;
};

export type RuntimeMailSettings = {
  enabled: boolean;
  smtp_host: string;
  smtp_port: number;
  username: string;
  password: string;
  from: string;
  reset_base_url: string;
  password_set: boolean;
  site_name?: string;
};
