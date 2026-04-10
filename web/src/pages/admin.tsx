import { useTranslation } from "react-i18next";

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";

export function AdminPage() {
  const { t } = useTranslation();

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("admin_console")}</CardTitle>
        <CardDescription>{t("admin_page_description")}</CardDescription>
      </CardHeader>
      <CardContent className="text-sm text-muted-foreground">
        {t("admin_page_content")}
      </CardContent>
    </Card>
  );
}
