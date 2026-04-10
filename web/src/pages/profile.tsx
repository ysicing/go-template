import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "../components/ui/card";
import { fetchCurrentUser } from "../lib/api";
import { ChangePasswordCard } from "../subsystems/auth/components/change-password-card";

function ProfileSummaryCard() {
  const { t } = useTranslation();
  const query = useQuery({
    queryKey: ["auth-me"],
    queryFn: fetchCurrentUser
  });

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("profile_summary_title")}</CardTitle>
        <CardDescription>{t("profile_summary_description")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-2 text-sm">
        {query.isLoading ? <div className="text-muted-foreground">{t("profile_loading")}</div> : null}
        <div>{t("profile_username", { value: query.data?.username ?? "-" })}</div>
        <div>{t("profile_email", { value: query.data?.email ?? "-" })}</div>
        <div>{t("profile_role", { value: query.data?.role ?? "-" })}</div>
      </CardContent>
    </Card>
  );
}

export function ProfilePage() {
  return (
    <div className="space-y-6">
      <ProfileSummaryCard />
      <ChangePasswordCard />
    </div>
  );
}
