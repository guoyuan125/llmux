"use client";

import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from "@/components/ui/table";
import {
  Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Plus, Trash2, Copy, Eye, EyeOff } from "lucide-react";
import { toast } from "sonner";
import { useI18n } from "@/lib/i18n";

interface APIKey {
  id: number;
  name: string;
  key: string;
  enabled: boolean;
  rpm_limit: number;
  tpm_limit: number;
  max_cost: number;
  expire_at: string;
  supported_models: string;
  created_at: string;
}

export default function APIKeysPage() {
  const { t } = useI18n();
  const [keys, setKeys] = useState<APIKey[]>([]);
  const [loading, setLoading] = useState(true);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [showKeys, setShowKeys] = useState<Record<number, boolean>>({});

  const [form, setForm] = useState({
    name: "",
    rpm_limit: 60,
    tpm_limit: 100000,
    max_cost: 0,
    supported_models: "",
  });

  const fetchKeys = () => {
    api<APIKey[]>("/api/apikeys")
      .then(setKeys)
      .catch(() => {})
      .finally(() => setLoading(false));
  };

  useEffect(() => { fetchKeys(); }, []);

  const handleCreate = async () => {
    try {
      await api("/api/apikeys", { method: "POST", body: JSON.stringify(form) });
      toast.success(t("apiKeys.created"));
      setDialogOpen(false);
      setForm({ name: "", rpm_limit: 60, tpm_limit: 100000, max_cost: 0, supported_models: "" });
      fetchKeys();
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : t("common.failed"));
    }
  };

  const handleDelete = async (id: number) => {
    if (!confirm(t("apiKeys.deleteConfirm"))) return;
    try {
      await api(`/api/apikeys/${id}`, { method: "DELETE" });
      toast.success(t("apiKeys.deleted"));
      fetchKeys();
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : t("common.failed"));
    }
  };

  const copyKey = (key: string) => {
    navigator.clipboard.writeText(key);
    toast.success(t("apiKeys.copied"));
  };

  const maskKey = (key: string) => {
    if (key.length <= 8) return "****";
    return key.slice(0, 4) + "..." + key.slice(-4);
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">{t("nav.apiKeys")}</h1>
          <p className="text-muted-foreground">{t("apiKeys.subtitle")}</p>
        </div>
        <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
          <DialogTrigger render={<Button />}>
            <Plus className="h-4 w-4 mr-2" />{t("apiKeys.createKey")}
          </DialogTrigger>
          <DialogContent className="max-w-lg">
            <DialogHeader>
              <DialogTitle>{t("apiKeys.createTitle")}</DialogTitle>
            </DialogHeader>
            <div className="grid gap-4 py-4">
              <div className="grid gap-2">
                <Label>{t("common.name")}</Label>
                <Input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} placeholder="my-app" />
              </div>
              <div className="grid grid-cols-2 gap-4">
                <div className="grid gap-2">
                  <Label>{t("apiKeys.rpmLimit")}</Label>
                  <Input type="number" value={form.rpm_limit} onChange={(e) => setForm({ ...form, rpm_limit: parseInt(e.target.value) || 0 })} />
                </div>
                <div className="grid gap-2">
                  <Label>{t("apiKeys.tpmLimit")}</Label>
                  <Input type="number" value={form.tpm_limit} onChange={(e) => setForm({ ...form, tpm_limit: parseInt(e.target.value) || 0 })} />
                </div>
              </div>
              <div className="grid gap-2">
                <Label>{t("apiKeys.allowedModels")}</Label>
                <Input value={form.supported_models} onChange={(e) => setForm({ ...form, supported_models: e.target.value })} placeholder="gpt-4o,claude-sonnet-4-20250514" />
              </div>
              <Button onClick={handleCreate}>{t("common.create")}</Button>
            </div>
          </DialogContent>
        </Dialog>
      </div>

      <Card>
        <CardHeader><CardTitle>{t("apiKeys.all")}</CardTitle></CardHeader>
        <CardContent>
          {loading ? (
            <div className="space-y-2">
              {[1, 2, 3].map((i) => <div key={i} className="h-12 rounded bg-muted animate-pulse" />)}
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t("common.name")}</TableHead>
                  <TableHead>{t("apiKeys.key")}</TableHead>
                  <TableHead>{t("common.status")}</TableHead>
                  <TableHead>{t("apiKeys.rpmTpm")}</TableHead>
                  <TableHead>{t("common.models")}</TableHead>
                  <TableHead className="text-right">{t("channels.actions")}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {keys.map((k) => (
                  <TableRow key={k.id}>
                    <TableCell className="font-medium">{k.name}</TableCell>
                    <TableCell>
                      <div className="flex items-center gap-1">
                        <code className="text-xs bg-muted px-1.5 py-0.5 rounded">
                          {showKeys[k.id] ? k.key : maskKey(k.key)}
                        </code>
                        <Button variant="ghost" size="icon-xs" onClick={() => setShowKeys({ ...showKeys, [k.id]: !showKeys[k.id] })}>
                          {showKeys[k.id] ? <EyeOff className="h-3 w-3" /> : <Eye className="h-3 w-3" />}
                        </Button>
                        <Button variant="ghost" size="icon-xs" onClick={() => copyKey(k.key)}>
                          <Copy className="h-3 w-3" />
                        </Button>
                      </div>
                    </TableCell>
                    <TableCell>
                      <Badge variant={k.enabled ? "default" : "secondary"}>
                        {k.enabled ? t("common.active") : t("common.disabled")}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-muted-foreground text-xs">
                      {k.rpm_limit} / {k.tpm_limit}
                    </TableCell>
                    <TableCell className="text-muted-foreground text-xs max-w-32 truncate">
                      {k.supported_models || t("common.all")}
                    </TableCell>
                    <TableCell className="text-right">
                      <Button variant="ghost" size="icon" onClick={() => handleDelete(k.id)}>
                        <Trash2 className="h-4 w-4 text-destructive" />
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
                {keys.length === 0 && (
                  <TableRow>
                    <TableCell colSpan={6} className="text-center text-muted-foreground py-8">
                      {t("apiKeys.empty")}
                    </TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
