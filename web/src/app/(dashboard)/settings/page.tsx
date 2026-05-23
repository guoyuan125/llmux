"use client";

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";

export default function SettingsPage() {
  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">Settings</h1>
        <p className="text-muted-foreground">Gateway configuration</p>
      </div>

      <div className="grid gap-4">
        <Card>
          <CardHeader>
            <CardTitle>Gateway Info</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="flex justify-between items-center">
              <span className="text-sm text-muted-foreground">Version</span>
              <Badge variant="secondary">v0.1.0</Badge>
            </div>
            <div className="flex justify-between items-center">
              <span className="text-sm text-muted-foreground">Database</span>
              <Badge variant="secondary">SQLite</Badge>
            </div>
            <div className="flex justify-between items-center">
              <span className="text-sm text-muted-foreground">Metrics</span>
              <Badge variant="default">Prometheus</Badge>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Endpoints</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="flex justify-between items-center">
              <span className="text-sm text-muted-foreground">OpenAI Compatible</span>
              <code className="text-xs bg-muted px-2 py-1 rounded">/v1/chat/completions</code>
            </div>
            <div className="flex justify-between items-center">
              <span className="text-sm text-muted-foreground">Anthropic Compatible</span>
              <code className="text-xs bg-muted px-2 py-1 rounded">/v1/messages</code>
            </div>
            <div className="flex justify-between items-center">
              <span className="text-sm text-muted-foreground">Models List</span>
              <code className="text-xs bg-muted px-2 py-1 rounded">/v1/models</code>
            </div>
            <div className="flex justify-between items-center">
              <span className="text-sm text-muted-foreground">Metrics</span>
              <code className="text-xs bg-muted px-2 py-1 rounded">/metrics</code>
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
