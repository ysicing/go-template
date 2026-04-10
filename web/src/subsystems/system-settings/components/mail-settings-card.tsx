import { useEffect, useState, type FormEvent } from "react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { fetchMailSettings, updateMailSettings } from "@/subsystems/system-settings/api/settings";
import type { RuntimeMailSettings } from "@/subsystems/system-settings/types";

function createEmptySettings(): RuntimeMailSettings {
  return {
    enabled: false,
    from: "",
    password: "",
    password_set: false,
    reset_base_url: "",
    site_name: "",
    smtp_host: "",
    smtp_port: 587,
    username: ""
  };
}

function getErrorMessage(error: unknown, fallbackMessage: string) {
  if (
    typeof error === "object" &&
    error !== null &&
    "response" in error &&
    typeof error.response === "object" &&
    error.response !== null &&
    "data" in error.response &&
    typeof error.response.data === "object" &&
    error.response.data !== null &&
    "message" in error.response.data &&
    typeof error.response.data.message === "string"
  ) {
    return error.response.data.message;
  }

  return error instanceof Error ? error.message : fallbackMessage;
}

export function MailSettingsCard() {
  const { t } = useTranslation();
  const query = useQuery({
    queryKey: ["system-mail-settings"],
    queryFn: fetchMailSettings
  });
  const [values, setValues] = useState<RuntimeMailSettings>(createEmptySettings);
  const [error, setError] = useState("");
  const [success, setSuccess] = useState("");

  useEffect(() => {
    if (query.data) {
      setValues({
        ...query.data,
        password: ""
      });
    }
  }, [query.data]);

  const mutation = useMutation({
    mutationFn: updateMailSettings,
    onSuccess: (data) => {
      setValues({
        ...data,
        password: ""
      });
      setError("");
      setSuccess(t("mail_settings_saved"));
    },
    onError: (submitError) => {
      setSuccess("");
      setError(getErrorMessage(submitError, t("mail_settings_save_failed")));
    }
  });

  function updateField<K extends keyof RuntimeMailSettings>(field: K, value: RuntimeMailSettings[K]) {
    setValues((current) => ({ ...current, [field]: value }));
    if (error) {
      setError("");
    }
    if (success) {
      setSuccess("");
    }
  }

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    mutation.mutate({
      ...values,
      from: values.from.trim(),
      password: values.password.trim(),
      reset_base_url: values.reset_base_url.trim(),
      smtp_host: values.smtp_host.trim(),
      username: values.username.trim()
    });
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("mail_settings_title")}</CardTitle>
        <CardDescription>{t("mail_settings_description")}</CardDescription>
      </CardHeader>
      <CardContent>
        {query.isLoading ? <div className="text-sm text-muted-foreground">{t("mail_settings_loading")}</div> : null}
        <form className="space-y-4" onSubmit={handleSubmit}>
          <div className="flex items-center justify-between rounded-lg border border-border px-4 py-3">
            <div className="space-y-1">
              <Label htmlFor="mail-enabled">{t("mail_settings_enabled")}</Label>
              <p className="text-sm text-muted-foreground">{t("mail_settings_enabled_hint")}</p>
            </div>
            <Switch
              checked={values.enabled}
              id="mail-enabled"
              onCheckedChange={(checked) => updateField("enabled", checked)}
            />
          </div>
          <div className="grid gap-4 md:grid-cols-2">
            <div className="space-y-2">
              <Label htmlFor="mail-smtp-host">{t("mail_settings_smtp_host")}</Label>
              <Input
                id="mail-smtp-host"
                onChange={(event) => updateField("smtp_host", event.target.value)}
                value={values.smtp_host}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="mail-smtp-port">{t("mail_settings_smtp_port")}</Label>
              <Input
                id="mail-smtp-port"
                min={1}
                onChange={(event) => updateField("smtp_port", Number(event.target.value) || 0)}
                type="number"
                value={values.smtp_port}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="mail-username">{t("mail_settings_username")}</Label>
              <Input
                id="mail-username"
                onChange={(event) => updateField("username", event.target.value)}
                value={values.username}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="mail-password">{t("mail_settings_password")}</Label>
              <Input
                id="mail-password"
                onChange={(event) => updateField("password", event.target.value)}
                placeholder={values.password_set ? t("mail_settings_password_placeholder") : ""}
                type="password"
                value={values.password}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="mail-from">{t("mail_settings_from")}</Label>
              <Input
                id="mail-from"
                onChange={(event) => updateField("from", event.target.value)}
                value={values.from}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="mail-reset-base-url">{t("mail_settings_reset_base_url")}</Label>
              <Input
                id="mail-reset-base-url"
                onChange={(event) => updateField("reset_base_url", event.target.value)}
                value={values.reset_base_url}
              />
            </div>
          </div>
          {error ? <p className="text-sm text-red-500">{error}</p> : null}
          {success ? <p className="text-sm text-green-600">{success}</p> : null}
          <div className="flex justify-end">
            <Button disabled={mutation.isPending || query.isLoading} type="submit">
              {mutation.isPending ? t("submitting") : t("submit")}
            </Button>
          </div>
        </form>
      </CardContent>
    </Card>
  );
}
