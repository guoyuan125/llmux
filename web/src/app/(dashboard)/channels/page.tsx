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
import { Plus, Pencil, Trash2, X, RefreshCw, Copy } from "lucide-react";
import { toast } from "sonner";
import { useI18n } from "@/lib/i18n";

interface Channel {
  id: number;
  name: string;
  type: string;
  enabled: boolean;
  base_urls: { id: number; url: string }[];
  keys: { id: number; key: string; enabled: boolean; status_code: number }[];
  proxy: string;
  param_override: string;
  models: string;
  custom_models: string;
}

const channelTypes = [
  { value: "openai", label: "OpenAI" },
  { value: "anthropic", label: "Anthropic" },
  { value: "gemini", label: "Gemini" },
];

function mergedModels(ch: Channel): string[] {
  const parts = [
    ...(ch.custom_models || "").split(","),
    ...(ch.models || "").split(","),
  ]
    .map((s) => s.trim())
    .filter(Boolean);
  return Array.from(new Set(parts));
}

export default function ChannelsPage() {
  const { t } = useI18n();
  const [channels, setChannels] = useState<Channel[]>([]);
  const [loading, setLoading] = useState(true);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editing, setEditing] = useState<Channel | null>(null);

  const [form, setForm] = useState({
    name: "",
    type: "openai",
    enabled: true,
    base_url: "",
    key: "",
    proxy: "",
    param_override: "",
    customModels: [] as string[],
  });

  const [newModel, setNewModel] = useState("");

  const [syncOpen, setSyncOpen] = useState(false);
  const [syncChannel, setSyncChannel] = useState<Channel | null>(null);
  const [syncAvailable, setSyncAvailable] = useState<string[]>([]);
  const [syncSelected, setSyncSelected] = useState<string[]>([]);
  const [syncLoading, setSyncLoading] = useState(false);

  const fetchChannels = () => {
    api<Channel[]>("/api/channels")
      .then(setChannels)
      .catch(() => {})
      .finally(() => setLoading(false));
  };

  useEffect(() => { fetchChannels(); }, []);

  const resetForm = () => {
    setForm({ name: "", type: "openai", enabled: true, base_url: "", key: "", proxy: "", param_override: "", customModels: [] });
    setNewModel("");
    setEditing(null);
  };

  const addModel = () => {
    const m = newModel.trim();
    if (!m || form.customModels.includes(m)) return;
    setForm((prev) => ({ ...prev, customModels: [...prev.customModels, m] }));
    setNewModel("");
  };

  const removeModel = (m: string) => {
    setForm((prev) => ({ ...prev, customModels: prev.customModels.filter((x) => x !== m) }));
  };

  const handleSync = async (ch: Channel) => {
    setSyncChannel(ch);
    setSyncAvailable([]);
    setSyncSelected(
      ch.custom_models
        ? ch.custom_models.split(",").map((s) => s.trim()).filter(Boolean)
        : []
    );
    setSyncOpen(true);
    setSyncLoading(true);
    try {
      const data = await api<{ models: string[] }>(`/api/channels/${ch.id}/sync-models`, { method: "POST" });
      const upstreamModels = data.models || [];
      setSyncAvailable((prev) => Array.from(new Set([...prev, ...upstreamModels])));
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : t("channels.syncFailed"));
      setSyncOpen(false);
    } finally {
      setSyncLoading(false);
    }
  };

  const handleSyncSave = async () => {
    if (!syncChannel) return;
    try {
      const payload = {
        name: syncChannel.name,
        type: syncChannel.type,
        enabled: syncChannel.enabled,
        base_urls: syncChannel.base_urls,
        keys: syncChannel.keys,
        proxy: syncChannel.proxy || "",
        param_override: syncChannel.param_override || "",
        custom_models: syncSelected.join(","),
      };
      await api(`/api/channels/${syncChannel.id}`, { method: "PUT", body: JSON.stringify(payload) });
      toast.success(t("channels.modelsSynced"));
      setSyncOpen(false);
      fetchChannels();
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : t("channels.saveFailed"));
    }
  };

  const handleSubmit = async () => {
    try {
      const payload = {
        name: form.name,
        type: form.type,
        enabled: form.enabled,
        base_urls: form.base_url ? [{ url: form.base_url }] : [],
        keys: form.key ? [{ key: form.key, enabled: true }] : [],
        proxy: form.proxy,
        param_override: form.param_override,
        custom_models: form.customModels.join(","),
      };

      if (editing) {
        await api(`/api/channels/${editing.id}`, { method: "PUT", body: JSON.stringify(payload) });
        toast.success(t("channels.updated"));
      } else {
        await api("/api/channels", { method: "POST", body: JSON.stringify(payload) });
        toast.success(t("channels.created"));
      }
      setDialogOpen(false);
      resetForm();
      fetchChannels();
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : t("common.failed"));
    }
  };

  const [deleteTarget, setDeleteTarget] = useState<Channel | null>(null);
  const [deleteGroups, setDeleteGroups] = useState<string[]>([]);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);

  const handleDelete = async (ch: Channel) => {
    try {
      const data = await api<{ groups: string[] }>(`/api/channels/${ch.id}?check=true`, { method: "DELETE" });
      setDeleteTarget(ch);
      setDeleteGroups(data.groups || []);
      setDeleteDialogOpen(true);
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : t("common.failed"));
    }
  };

  const confirmDelete = async () => {
    if (!deleteTarget) return;
    try {
      await api(`/api/channels/${deleteTarget.id}`, { method: "DELETE" });
      toast.success(t("channels.deleted"));
      setDeleteDialogOpen(false);
      setDeleteTarget(null);
      fetchChannels();
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : t("common.failed"));
    }
  };

  const handleDuplicate = async (ch: Channel) => {
    try {
      await api(`/api/channels/${ch.id}/duplicate`, { method: "POST" });
      toast.success(t("channels.duplicated"));
      fetchChannels();
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : t("common.failed"));
    }
  };

  const handleEdit = (ch: Channel) => {
    setEditing(ch);
    setForm({
      name: ch.name,
      type: ch.type,
      enabled: ch.enabled,
      base_url: ch.base_urls?.[0]?.url || "",
      key: ch.keys?.[0]?.key || "",
      proxy: ch.proxy || "",
      param_override: ch.param_override || "",
      customModels: ch.custom_models
        ? ch.custom_models.split(",").map((s) => s.trim()).filter(Boolean)
        : [],
    });
    setDialogOpen(true);
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">{t("common.channels")}</h1>
          <p className="text-muted-foreground">{t("channels.subtitle")}</p>
        </div>
        <Dialog open={dialogOpen} onOpenChange={(open) => { setDialogOpen(open); if (!open) resetForm(); }}>
          <DialogTrigger render={<Button />}>
            <Plus className="h-4 w-4 mr-2" />{t("channels.add")}
          </DialogTrigger>
          <DialogContent className="max-w-lg">
            <DialogHeader>
              <DialogTitle>{editing ? t("channels.edit") : t("channels.new")}</DialogTitle>
            </DialogHeader>
            <div className="grid gap-4 py-4">
              <div className="grid gap-2">
                <Label>{t("common.name")}</Label>
                <Input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} placeholder="my-openai" />
              </div>
              <div className="grid gap-2">
                <Label>{t("channels.type")}</Label>
                <select
                  className="flex h-8 w-full rounded-lg border border-input bg-transparent px-3 py-1 text-sm transition-colors focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
                  value={form.type}
                  onChange={(e) => setForm({ ...form, type: e.target.value })}
                >
                  {channelTypes.map((t) => (
                    <option key={t.value} value={t.value}>{t.label}</option>
                  ))}
                </select>
              </div>
              <div className="grid gap-2">
                <Label>{t("channels.baseUrl")}</Label>
                <Input value={form.base_url} onChange={(e) => setForm({ ...form, base_url: e.target.value })} placeholder="https://api.openai.com" />
              </div>
              <div className="grid gap-2">
                <Label>{t("channels.apiKey")}</Label>
                <Input value={form.key} onChange={(e) => setForm({ ...form, key: e.target.value })} placeholder="sk-..." type="password" />
              </div>
              <div className="grid gap-2">
                <Label>{t("channels.proxy")}</Label>
                <Input value={form.proxy} onChange={(e) => setForm({ ...form, proxy: e.target.value })} placeholder="http://proxy:8080" />
              </div>
              <div className="grid gap-2">
                <Label>{t("common.models")}</Label>
                <div className="flex flex-wrap gap-1 min-h-[32px] rounded-lg border border-input bg-transparent px-2 py-1.5">
                  {form.customModels.length > 0 ? (
                    form.customModels.map((m) => (
                      <span
                        key={m}
                        className="inline-flex items-center gap-0.5 rounded-md bg-secondary px-2 py-0.5 text-xs font-mono"
                      >
                        {m}
                        <button
                          type="button"
                          onClick={() => removeModel(m)}
                          className="ml-0.5 opacity-60 hover:opacity-100"
                          aria-label={t("channels.removeModel", { name: m })}
                        >
                          <X className="h-3 w-3" />
                        </button>
                      </span>
                    ))
                  ) : (
                    <span className="text-xs text-muted-foreground self-center">{t("channels.noModelsConfigured")}</span>
                  )}
                </div>
                <div className="flex gap-2">
                  <Input
                    value={newModel}
                    onChange={(e) => setNewModel(e.target.value)}
                    onKeyDown={(e) => { if (e.key === "Enter") { e.preventDefault(); addModel(); } }}
                    placeholder="e.g. gpt-4o"
                    className="h-8 text-sm font-mono"
                  />
                  <Button type="button" variant="outline" size="sm" onClick={addModel} className="h-8 shrink-0">
                    <Plus className="h-3.5 w-3.5 mr-1" />{t("common.add")}
                  </Button>
                </div>
                <p className="text-xs text-muted-foreground">
                  {t("channels.addModelHelp")}
                </p>
              </div>
              <Button onClick={handleSubmit}>{editing ? t("common.update") : t("common.create")}</Button>
            </div>
          </DialogContent>
        </Dialog>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>{t("channels.all")}</CardTitle>
        </CardHeader>
        <CardContent>
          {loading ? (
            <div className="space-y-2">
              {[1, 2, 3].map((i) => <div key={i} className="h-12 rounded bg-muted animate-pulse" />)}
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="w-36">{t("common.name")}</TableHead>
                  <TableHead className="w-24">{t("channels.type")}</TableHead>
                  <TableHead className="w-40">{t("channels.baseUrl")}</TableHead>
                  <TableHead className="w-20">{t("common.status")}</TableHead>
                  <TableHead className="w-12">{t("channels.keys")}</TableHead>
                  <TableHead>{t("common.models")}</TableHead>
                  <TableHead className="text-right w-36">{t("channels.actions")}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {channels.map((ch) => {
                  const models = mergedModels(ch);
                  return (
                  <TableRow key={ch.id}>
                    <TableCell className="font-medium max-w-36 truncate">{ch.name}</TableCell>
                    <TableCell><Badge variant="secondary">{ch.type}</Badge></TableCell>
                    <TableCell className="text-muted-foreground text-xs w-40 max-w-40 truncate">
                      {ch.base_urls?.[0]?.url || "-"}
                    </TableCell>
                    <TableCell>
                      <Badge variant={ch.enabled ? "default" : "secondary"}>
                        {ch.enabled ? t("common.active") : t("common.disabled")}
                      </Badge>
                    </TableCell>
                    <TableCell>{ch.keys?.length || 0}</TableCell>
                    <TableCell className="max-w-48">
                      <div className="flex flex-wrap gap-1">
                        {models.length > 0 ? (
                          <>
                            {models.slice(0, 2).map((m) => (
                              <Badge key={m} variant="outline" className="text-xs font-mono max-w-32 truncate">
                                {m}
                              </Badge>
                            ))}
                            {models.length > 2 && (
                              <Badge variant="outline" className="text-xs text-muted-foreground">
                                +{models.length - 2}
                              </Badge>
                            )}
                          </>
                        ) : (
                          <span className="text-xs text-muted-foreground">{t("channels.noModels")}</span>
                        )}
                      </div>
                    </TableCell>
                    <TableCell className="text-right">
                      <Button variant="ghost" size="icon" onClick={() => handleSync(ch)} title={t("common.sync")}>
                        <RefreshCw className="h-4 w-4" />
                      </Button>
                      <Button variant="ghost" size="icon" onClick={() => handleDuplicate(ch)} title={t("common.duplicate")}>
                        <Copy className="h-4 w-4" />
                      </Button>
                      <Button variant="ghost" size="icon" onClick={() => handleEdit(ch)} title={t("common.edit")}>
                        <Pencil className="h-4 w-4" />
                      </Button>
                      <Button variant="ghost" size="icon" onClick={() => handleDelete(ch)} title={t("common.delete")}>
                        <Trash2 className="h-4 w-4 text-destructive" />
                      </Button>
                    </TableCell>
                  </TableRow>
                  );
                })}
                {channels.length === 0 && (
                  <TableRow>
                    <TableCell colSpan={7} className="text-center text-muted-foreground py-8">
                      {t("channels.empty")}
                    </TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      <Dialog open={deleteDialogOpen} onOpenChange={(open) => { setDeleteDialogOpen(open); if (!open) setDeleteTarget(null); }}>
        <DialogContent className="max-w-sm">
          <DialogHeader>
            <DialogTitle>{t("channels.deleteTitle")}</DialogTitle>
          </DialogHeader>
          <div className="space-y-3 py-2">
            {deleteGroups.length > 0 ? (
              <>
                <p className="text-sm">{t("channels.deleteUsed", { name: deleteTarget?.name ?? "" })}</p>
                <ul className="text-sm space-y-1 pl-4">
                  {deleteGroups.map((g) => (
                    <li key={g} className="list-disc text-muted-foreground font-mono">{g}</li>
                  ))}
                </ul>
                <p className="text-sm text-muted-foreground">{t("channels.deleteUsedHint")}</p>
              </>
            ) : (
              <p className="text-sm">{t("channels.deleteConfirm", { name: deleteTarget?.name ?? "" })}</p>
            )}
            <div className="flex justify-end gap-2 pt-1">
              <Button variant="outline" size="sm" onClick={() => setDeleteDialogOpen(false)}>{t("common.cancel")}</Button>
              <Button variant="destructive" size="sm" onClick={confirmDelete}>{t("common.delete")}</Button>
            </div>
          </div>
        </DialogContent>
      </Dialog>

      <Dialog open={syncOpen} onOpenChange={setSyncOpen}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>{t("channels.syncTitle", { name: syncChannel?.name ?? "" })}</DialogTitle>
          </DialogHeader>
          {syncLoading ? (
            <div className="space-y-2 py-4">
              {[1, 2, 3].map((i) => <div key={i} className="h-8 rounded bg-muted animate-pulse" />)}
            </div>
          ) : (
            <div className="space-y-4 py-2">
              {syncAvailable.length === 0 ? (
                <p className="text-sm text-muted-foreground">{t("channels.noUpstreamModels")}</p>
              ) : (
                <div className="space-y-2 max-h-64 overflow-y-auto pr-1">
                  {syncAvailable.map((m) => (
                    <label key={m} className="flex items-center gap-2 cursor-pointer text-sm font-mono">
                      <input
                        type="checkbox"
                        checked={syncSelected.includes(m)}
                        onChange={(e) => {
                          if (e.target.checked) {
                            setSyncSelected((prev) => [...prev, m]);
                          } else {
                            setSyncSelected((prev) => prev.filter((x) => x !== m));
                          }
                        }}
                        className="h-4 w-4"
                      />
                      {m}
                    </label>
                  ))}
                </div>
              )}
              <div className="flex justify-end gap-2 pt-2">
                <Button variant="outline" size="sm" onClick={() => setSyncOpen(false)}>{t("common.cancel")}</Button>
                <Button size="sm" onClick={handleSyncSave} disabled={syncAvailable.length === 0}>
                  {t("channels.saveSelected", { count: syncSelected.length })}
                </Button>
              </div>
            </div>
          )}
        </DialogContent>
      </Dialog>
    </div>
  );
}
