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
import { Plus, Pencil, Trash2, X } from "lucide-react";
import { toast } from "sonner";

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
        toast.success("Channel updated");
      } else {
        await api("/api/channels", { method: "POST", body: JSON.stringify(payload) });
        toast.success("Channel created");
      }
      setDialogOpen(false);
      resetForm();
      fetchChannels();
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : "Failed");
    }
  };

  const handleDelete = async (id: number) => {
    if (!confirm("Delete this channel?")) return;
    try {
      await api(`/api/channels/${id}`, { method: "DELETE" });
      toast.success("Channel deleted");
      fetchChannels();
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : "Failed");
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
          <h1 className="text-2xl font-bold tracking-tight">Channels</h1>
          <p className="text-muted-foreground">Manage upstream LLM provider channels</p>
        </div>
        <Dialog open={dialogOpen} onOpenChange={(open) => { setDialogOpen(open); if (!open) resetForm(); }}>
          <DialogTrigger render={<Button />}>
            <Plus className="h-4 w-4 mr-2" />Add Channel
          </DialogTrigger>
          <DialogContent className="max-w-lg">
            <DialogHeader>
              <DialogTitle>{editing ? "Edit Channel" : "New Channel"}</DialogTitle>
            </DialogHeader>
            <div className="grid gap-4 py-4">
              <div className="grid gap-2">
                <Label>Name</Label>
                <Input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} placeholder="my-openai" />
              </div>
              <div className="grid gap-2">
                <Label>Type</Label>
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
                <Label>Base URL</Label>
                <Input value={form.base_url} onChange={(e) => setForm({ ...form, base_url: e.target.value })} placeholder="https://api.openai.com" />
              </div>
              <div className="grid gap-2">
                <Label>API Key</Label>
                <Input value={form.key} onChange={(e) => setForm({ ...form, key: e.target.value })} placeholder="sk-..." type="password" />
              </div>
              <div className="grid gap-2">
                <Label>Proxy (optional)</Label>
                <Input value={form.proxy} onChange={(e) => setForm({ ...form, proxy: e.target.value })} placeholder="http://proxy:8080" />
              </div>
              <div className="grid gap-2">
                <Label>Models</Label>
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
                          aria-label={`Remove ${m}`}
                        >
                          <X className="h-3 w-3" />
                        </button>
                      </span>
                    ))
                  ) : (
                    <span className="text-xs text-muted-foreground self-center">No models configured</span>
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
                    <Plus className="h-3.5 w-3.5 mr-1" />Add
                  </Button>
                </div>
                <p className="text-xs text-muted-foreground">
                  Custom model names exposed by this channel. Press Enter or click Add.
                </p>
              </div>
              <Button onClick={handleSubmit}>{editing ? "Update" : "Create"}</Button>
            </div>
          </DialogContent>
        </Dialog>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>All Channels</CardTitle>
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
                  <TableHead>Name</TableHead>
                  <TableHead>Type</TableHead>
                  <TableHead>Base URL</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Keys</TableHead>
                  <TableHead>Models</TableHead>
                  <TableHead className="text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {channels.map((ch) => (
                  <TableRow key={ch.id}>
                    <TableCell className="font-medium">{ch.name}</TableCell>
                    <TableCell><Badge variant="secondary">{ch.type}</Badge></TableCell>
                    <TableCell className="text-muted-foreground text-xs max-w-48 truncate">
                      {ch.base_urls?.[0]?.url || "-"}
                    </TableCell>
                    <TableCell>
                      <Badge variant={ch.enabled ? "default" : "secondary"}>
                        {ch.enabled ? "Active" : "Disabled"}
                      </Badge>
                    </TableCell>
                    <TableCell>{ch.keys?.length || 0}</TableCell>
                    <TableCell>
                      <div className="flex flex-wrap gap-1">
                        {mergedModels(ch).length > 0 ? (
                          mergedModels(ch).map((m) => (
                            <Badge key={m} variant="outline" className="text-xs font-mono">
                              {m}
                            </Badge>
                          ))
                        ) : (
                          <span className="text-xs text-muted-foreground">No models</span>
                        )}
                      </div>
                    </TableCell>
                    <TableCell className="text-right">
                      <Button variant="ghost" size="icon" onClick={() => handleEdit(ch)}>
                        <Pencil className="h-4 w-4" />
                      </Button>
                      <Button variant="ghost" size="icon" onClick={() => handleDelete(ch.id)}>
                        <Trash2 className="h-4 w-4 text-destructive" />
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
                {channels.length === 0 && (
                  <TableRow>
                    <TableCell colSpan={7} className="text-center text-muted-foreground py-8">
                      No channels yet. Add your first channel to get started.
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
