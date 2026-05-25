"use client";

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { useI18n } from "@/lib/i18n";

export default function SettingsPage() {
  const { locale, setLocale, t } = useI18n();

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">{t("nav.settings")}</h1>
        <p className="text-muted-foreground">{t("settings.subtitle")}</p>
      </div>

      <div className="grid gap-4">
        <Card>
          <CardHeader>
            <CardTitle>{t("settings.interface")}</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="flex items-center justify-between gap-4">
              <div>
                <span className="text-sm text-muted-foreground">{t("common.language")}</span>
                <p className="text-xs text-muted-foreground mt-1">{t("settings.languageHelp")}</p>
              </div>
              <div className="flex rounded-md bg-muted p-0.5">
                <Button
                  variant={locale === "en" ? "default" : "ghost"}
                  size="sm"
                  className="h-7 text-xs px-3"
                  onClick={() => setLocale("en")}
                >
                  {t("settings.english")}
                </Button>
                <Button
                  variant={locale === "zh" ? "default" : "ghost"}
                  size="sm"
                  className="h-7 text-xs px-3"
                  onClick={() => setLocale("zh")}
                >
                  {t("settings.chinese")}
                </Button>
              </div>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>{t("settings.gatewayInfo")}</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="flex justify-between items-center">
              <span className="text-sm text-muted-foreground">{t("settings.version")}</span>
              <Badge variant="secondary">v0.1.0</Badge>
            </div>
            <div className="flex justify-between items-center">
              <span className="text-sm text-muted-foreground">{t("settings.database")}</span>
              <Badge variant="secondary">SQLite</Badge>
            </div>
            <div className="flex justify-between items-center">
              <span className="text-sm text-muted-foreground">{t("settings.metrics")}</span>
              <Badge variant="default">Prometheus</Badge>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>{t("settings.endpoints")}</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="flex justify-between items-center">
              <span className="text-sm text-muted-foreground">{t("settings.openaiCompatible")}</span>
              <code className="text-xs bg-muted px-2 py-1 rounded">/v1/chat/completions</code>
            </div>
            <div className="flex justify-between items-center">
              <span className="text-sm text-muted-foreground">{t("settings.anthropicCompatible")}</span>
              <code className="text-xs bg-muted px-2 py-1 rounded">/v1/messages</code>
            </div>
            <div className="flex justify-between items-center">
              <span className="text-sm text-muted-foreground">{t("settings.modelsList")}</span>
              <code className="text-xs bg-muted px-2 py-1 rounded">/v1/models</code>
            </div>
            <div className="flex justify-between items-center">
              <span className="text-sm text-muted-foreground">{t("settings.metrics")}</span>
              <code className="text-xs bg-muted px-2 py-1 rounded">/metrics</code>
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
