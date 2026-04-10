import { Sparkles } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";

export function HomePage() {
  const { t } = useTranslation();

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <Sparkles className="h-5 w-5 text-accent" />
          {t("title")}
        </CardTitle>
        <CardDescription>{t("home_intro")}</CardDescription>
      </CardHeader>
      <CardContent className="text-sm text-muted-foreground">
        {t("home_stack")}
      </CardContent>
    </Card>
  );
}
