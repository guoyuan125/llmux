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
import { Plus, Pencil, Trash2, X, ArrowUp, ArrowDown } from "lucide-react";
import { toast } from "sonner";

interface ChannelURL {
  url: string;
}

interface Channel {
  id: number;
  name: string;
  type: number;
  base_urls: ChannelURL[];
}

interface GroupItem {
  channel_id: number;
  model_name: string;
  priority: number;
  weight: number;
}

interface Group {
  id: number;
  name: string;
  models: string;
  mode: string;
  context_size: number;
  session_keep_time: number;
  first_token_timeout: number;
  items: GroupItem[];
}

interface CircuitEntry {
  key: string;
  channel_id: number;
  state: "closed" | "open" | "half_open";
  failures: number;
  threshold: number;
  last_failure: string;
  next_retry: string;
}

const modes = [
  { value: "round_robin", label: "Round Robin" },
  { value: "random", label: "Random" },
  { value: "failover", label: "Failover (Priority)" },
  { value: "weighted", label: "Weighted" },
  { value: "least_cost", label: "Least Cost" },
  { value: "least_latency", label: "Least Latency" },
];

const channelTypeLabel: Record<number, string> = {
  1: "OpenAI",
  2: "Anthropic",
  3: "Gemini",
};

const emptyItem = (): GroupItem => ({ channel_id: 0, model_name: "", priority: 1, weight: 1 });

type ChannelStatus =
  | { kind: "running" }
  | { kind: "ready" }
  | { kind: "tripped"; secsLeft: number }
  | { kind: "testing" };

function getChannelStatus(
  items: GroupItem[],
  idx: number,
  circuitMap: Record<number, CircuitEntry>,
  groupMode: string,
  now: number
): ChannelStatus {
  const cb = circuitMap[items[idx].channel_id];
  if (cb?.state === "open") {
    const secsLeft = cb.next_retry
      ? Math.max(0, Math.ceil((new Date(cb.next_retry).getTime() - now) / 1000))
      : 0;
    return { kind: "tripped", secsLeft };
  }
  if (cb?.state === "half_open") return { kind: "testing" };
  // healthy (closed or no entry)
  if (groupMode === "failover") {
    const firstHealthyIdx = items.findIndex(
      (it) => circuitMap[it.channel_id]?.state !== "open"
    );
    return idx === firstHealthyIdx ? { kind: "running" } : { kind: "ready" };
  }
  return { kind: "running" };
}

export default function GroupsPage() {
  const [groups, setGroups] = useState<Group[]>([]);
  const [channels, setChannels] = useState<Channel[]>([]);
  const [circuitMap, setCircuitMap] = useState<Record<number, CircuitEntry>>({});
  const [loading, setLoading] = useState(true);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editing, setEditing] = useState<Group | null>(null);
  const [tick, setTick] = useState(0);

  const [form, setForm] = useState({
    name: "",
    models: "",
    mode: "round_robin",
    context_size: 0,
    session_keep_time: 0,
    first_token_timeout: 0,
  });
  const [items, setItems] = useState<GroupItem[]>([emptyItem()]);

  const fetchCircuit = () => {
    api<CircuitEntry[]>("/api/circuit/status")
      .then((entries) => {
        const m: Record<number, CircuitEntry> = {};
        for (const e of entries) m[e.channel_id] = e;
        setCircuitMap(m);
      })
      .catch(() => {});
  };

  const fetchGroups = () => {
    api<Group[]>("/api/groups")
      .then(setGroups)
      .catch(() => {})
      .finally(() => setLoading(false));
  };

  useEffect(() => {
    fetchGroups();
    api<Channel[]>("/api/channels").then(setChannels).catch(() => {});
    fetchCircuit();
    const timer = setInterval(fetchCircuit, 5000);
    return () => clearInterval(timer);
  }, []);

  useEffect(() => {
    const t = setInterval(() => setTick((n) => n + 1), 1000);
    return () => clearInterval(t);
  }, []);

  const resetForm = () => {
    setForm({ name: "", models: "", mode: "round_robin", context_size: 0, session_keep_time: 0, first_token_timeout: 0 });
    setItems([emptyItem()]);
    setEditing(null);
  };

  const moveItem = (idx: number, dir: -1 | 1) => {
    const next = idx + dir;
    if (next < 0 || next >= items.length) return;
    setItems((prev) => {
      const arr = [...prev];
      [arr[idx], arr[next]] = [arr[next], arr[idx]];
      return arr;
    });
  };

  const handleSubmit = async () => {
    const validItems = items
      .filter((it) => it.channel_id > 0 && it.model_name.trim())
      .map((it, i) => ({ ...it, priority: i + 1, weight: it.weight || 1 }));
    try {
      const payload = { ...form, items: validItems };
      if (editing) {
        await api(`/api/groups/${editing.id}`, { method: "PUT", body: JSON.stringify(payload) });
        toast.success("Group updated");
      } else {
        await api("/api/groups", { method: "POST", body: JSON.stringify(payload) });
        toast.success("Group created");
      }
      setDialogOpen(false);
      resetForm();
      fetchGroups();
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : "Failed");
    }
  };

  const handleDelete = async (id: number) => {
    if (!confirm("Delete this group?")) return;
    try {
      await api(`/api/groups/${id}`, { method: "DELETE" });
      toast.success("Group deleted");
      fetchGroups();
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : "Failed");
    }
  };

  const handleEdit = (g: Group) => {
    setEditing(g);
    setForm({
      name: g.name,
      models: g.models || "",
      mode: g.mode,
      context_size: g.context_size || 0,
      session_keep_time: g.session_keep_time || 0,
      first_token_timeout: g.first_token_timeout || 0,
    });
    setItems(g.items?.length ? g.items.map((it) => ({ ...it })) : [emptyItem()]);
    setDialogOpen(true);
  };

  const updateItem = (idx: number, patch: Partial<GroupItem>) => {
    setItems((prev) => prev.map((it, i) => (i === idx ? { ...it, ...patch } : it)));
  };

  const removeItem = (idx: number) => {
    setItems((prev) => prev.filter((_, i) => i !== idx));
  };

  const selectedChannel = (id: number) => channels.find((c) => c.id === id);

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">Groups</h1>
          <p className="text-muted-foreground">Model routing groups with load balancing</p>
        </div>
        <Dialog key={editing?.id ?? 'new'} open={dialogOpen} onOpenChange={(open) => { setDialogOpen(open); if (!open) resetForm(); }}>
          <DialogTrigger render={<Button />}>
            <Plus className="h-4 w-4 mr-2" />Add Group
          </DialogTrigger>

          {/* sm:max-w-4xl overrides the sm:max-w-sm in DialogContent base styles */}
          <DialogContent className="sm:max-w-4xl">
            <DialogHeader>
              <DialogTitle className="text-lg">{editing ? "Edit Group" : "New Group"}</DialogTitle>
            </DialogHeader>

            <div className="space-y-6 pt-2">
              {/* Section: Basic Settings */}
              <div className="space-y-4">
                <h3 className="text-sm font-semibold text-muted-foreground uppercase tracking-wide">Basic Settings</h3>
                <div className="grid grid-cols-2 gap-4">
                  <div className="space-y-2">
                    <Label htmlFor="group-name">Group Name</Label>
                    <Input
                      id="group-name"
                      value={form.name}
                      onChange={(e) => setForm({ ...form, name: e.target.value })}
                      placeholder="e.g. internal"
                      className="h-9"
                    />
                    <p className="text-xs text-muted-foreground">Display name for management</p>
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="group-mode">Balancing Mode</Label>
                    <select
                      id="group-mode"
                      className="flex h-9 w-full rounded-lg border border-input bg-transparent px-3 py-1 text-sm transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                      value={form.mode}
                      onChange={(e) => setForm({ ...form, mode: e.target.value })}
                    >
                      {modes.map((m) => (
                        <option key={m.value} value={m.value}>{m.label}</option>
                      ))}
                    </select>
                    <p className="text-xs text-muted-foreground">How to pick a channel when multiple are available</p>
                  </div>
                </div>
                <div className="space-y-2">
                  <Label htmlFor="group-models">Accepted Models</Label>
                  <Input
                    id="group-models"
                    value={form.models}
                    onChange={(e) => setForm({ ...form, models: e.target.value })}
                    placeholder="e.g. internal, gpt-4o, claude-sonnet-4-5"
                    className="h-9"
                  />
                  <p className="text-xs text-muted-foreground">Comma-separated exact model names that route to this group.</p>
                </div>
                <div className="grid grid-cols-3 gap-4">
                  <div className="space-y-2">
                    <Label htmlFor="ctx">Context Size (tokens)</Label>
                    <Input
                      id="ctx"
                      type="number"
                      value={form.context_size}
                      onChange={(e) => setForm({ ...form, context_size: parseInt(e.target.value) || 0 })}
                      className="h-9"
                    />
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="skt">Session Keep (sec)</Label>
                    <Input
                      id="skt"
                      type="number"
                      value={form.session_keep_time}
                      onChange={(e) => setForm({ ...form, session_keep_time: parseInt(e.target.value) || 0 })}
                      className="h-9"
                    />
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="ftt">First Token Timeout (sec)</Label>
                    <Input
                      id="ftt"
                      type="number"
                      value={form.first_token_timeout}
                      onChange={(e) => setForm({ ...form, first_token_timeout: parseInt(e.target.value) || 0 })}
                      className="h-9"
                    />
                  </div>
                </div>
              </div>

              {/* Section: Channels */}
              <div className="space-y-4">
                <div className="flex items-center justify-between">
                  <div>
                    <h3 className="text-sm font-semibold text-muted-foreground uppercase tracking-wide">Channels</h3>
                    <p className="text-xs text-muted-foreground mt-0.5">
                      Each entry maps this group to an upstream channel and model. List order determines priority (top = highest).
                    </p>
                  </div>
                  <Button variant="outline" size="sm" onClick={() => setItems((prev) => [...prev, emptyItem()])}>
                    <Plus className="h-3.5 w-3.5 mr-1.5" />Add Channel
                  </Button>
                </div>

                <div className="max-h-[260px] overflow-y-auto space-y-3 pr-1">
                  {items.map((it, idx) => {
                    const ch = selectedChannel(it.channel_id);
                    const baseUrl = ch?.base_urls?.[0]?.url ?? "";
                    return (
                      <div key={idx} className="rounded-lg border border-border bg-muted/20 p-4 space-y-3">
                        {/* Item header */}
                        <div className="flex items-center justify-between">
                          <span className="text-xs font-medium text-muted-foreground">Channel {idx + 1}</span>
                          <div className="flex items-center gap-1">
                            <Button
                              variant="ghost"
                              size="icon"
                              className="h-6 w-6 text-muted-foreground"
                              onClick={() => moveItem(idx, -1)}
                              disabled={idx === 0}
                            >
                              <ArrowUp className="h-3.5 w-3.5" />
                            </Button>
                            <Button
                              variant="ghost"
                              size="icon"
                              className="h-6 w-6 text-muted-foreground"
                              onClick={() => moveItem(idx, 1)}
                              disabled={idx === items.length - 1}
                            >
                              <ArrowDown className="h-3.5 w-3.5" />
                            </Button>
                            <Button
                              variant="ghost"
                              size="icon"
                              className="h-6 w-6 text-muted-foreground hover:text-destructive"
                              onClick={() => removeItem(idx)}
                              disabled={items.length === 1}
                            >
                              <X className="h-3.5 w-3.5" />
                            </Button>
                          </div>
                        </div>

                        {/* Row 1: channel select (full width) */}
                        <div className="space-y-1.5">
                          <Label className="text-xs">Channel</Label>
                          <select
                            className="flex h-9 w-full rounded-md border border-input bg-background px-3 py-1 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                            value={it.channel_id}
                            onChange={(e) => updateItem(idx, { channel_id: parseInt(e.target.value) })}
                          >
                            <option value={0}>— select channel —</option>
                            {channels.map((c) => (
                              <option key={c.id} value={c.id}>
                                {c.name}  ({c.base_urls?.[0]?.url ?? "no url"})
                              </option>
                            ))}
                          </select>
                          {/* show selected channel detail below */}
                          {ch && (
                            <div className="flex items-center gap-2 px-1">
                              <Badge variant="secondary" className="text-xs">
                                {channelTypeLabel[ch.type] ?? `type ${ch.type}`}
                              </Badge>
                              <span className="text-xs text-muted-foreground truncate">{baseUrl}</span>
                            </div>
                          )}
                        </div>

                        {/* Row 2: model name only */}
                        <div className="grid grid-cols-[1fr] gap-3">
                          <div className="space-y-1.5">
                            <Label className="text-xs">Upstream Model Name</Label>
                            <Input
                              placeholder="e.g. claude-sonnet-4-5"
                              value={it.model_name}
                              onChange={(e) => updateItem(idx, { model_name: e.target.value })}
                              className="h-9"
                            />
                          </div>
                        </div>
                      </div>
                    );
                  })}

                  {items.length === 0 && (
                    <div className="rounded-lg border border-dashed p-8 text-center text-sm text-muted-foreground">
                      No channels added. Click &ldquo;Add Channel&rdquo; above.
                    </div>
                  )}
                </div>
              </div>

              <Button onClick={handleSubmit} className="w-full h-10">
                {editing ? "Update Group" : "Create Group"}
              </Button>
            </div>
          </DialogContent>
        </Dialog>
      </div>

      {/* Table */}
      <Card>
        <CardHeader><CardTitle>All Groups</CardTitle></CardHeader>
        <CardContent>
          {loading ? (
            <div className="space-y-2">
              {[1, 2, 3].map((i) => <div key={i} className="h-12 rounded bg-muted animate-pulse" />)}
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="w-[130px]">Name</TableHead>
                  <TableHead className="w-[200px]">Accepted Models</TableHead>
                  <TableHead className="w-[130px]">Mode</TableHead>
                  <TableHead>Channels</TableHead>
                  <TableHead className="w-[90px] text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {groups.map((g) => (
                  <TableRow key={g.id}>
                    <TableCell className="font-medium">{g.name}</TableCell>
                    <TableCell>
                      <div className="flex flex-wrap gap-1">
                        {(g.models || "").split(",").filter(Boolean).map((m, i) => (
                          <Badge key={i} variant="outline" className="text-xs font-mono">{m.trim()}</Badge>
                        ))}
                      </div>
                    </TableCell>
                    <TableCell><Badge variant="secondary">{g.mode}</Badge></TableCell>
                    <TableCell>
                      <div className="flex flex-wrap gap-1.5">
                        {g.items?.map((it, i) => {
                          const ch = channels.find((c) => c.id === it.channel_id);
                          const status = getChannelStatus(g.items, i, circuitMap, g.mode, tick > 0 ? Date.now() : Date.now());
                          return (
                            <div key={i} className="flex items-center gap-1.5 rounded-md border border-border bg-muted/40 px-2 py-1 text-xs">
                              <span className="font-medium">{ch?.name ?? `#${it.channel_id}`}</span>
                              <span className="text-muted-foreground">→</span>
                              <span>{it.model_name}</span>
                              {status.kind === "running" && (
                                <span className="inline-flex items-center gap-0.5 text-emerald-600 dark:text-emerald-400">
                                  <span className="h-1.5 w-1.5 rounded-full bg-emerald-500 inline-block" />
                                  Running
                                </span>
                              )}
                              {status.kind === "ready" && (
                                <span className="inline-flex items-center gap-0.5 text-sky-600 dark:text-sky-400">
                                  <span className="h-1.5 w-1.5 rounded-full bg-sky-400 inline-block" />
                                  Ready
                                </span>
                              )}
                              {status.kind === "testing" && (
                                <span className="inline-flex items-center gap-0.5 text-amber-600 dark:text-amber-400">
                                  <span className="h-1.5 w-1.5 rounded-full bg-amber-500 inline-block" />
                                  Testing
                                </span>
                              )}
                              {status.kind === "tripped" && (
                                <span className="inline-flex items-center gap-0.5 text-destructive">
                                  <span className="h-1.5 w-1.5 rounded-full bg-destructive inline-block" />
                                  Tripped · {status.secsLeft}s
                                </span>
                              )}
                            </div>
                          );
                        })}
                        {(!g.items || g.items.length === 0) && (
                          <span className="text-muted-foreground text-xs">none</span>
                        )}
                      </div>
                    </TableCell>
                    <TableCell className="text-right">
                      <Button variant="ghost" size="icon" onClick={() => handleEdit(g)}>
                        <Pencil className="h-4 w-4" />
                      </Button>
                      <Button variant="ghost" size="icon" onClick={() => handleDelete(g.id)}>
                        <Trash2 className="h-4 w-4 text-destructive" />
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
                {groups.length === 0 && (
                  <TableRow>
                    <TableCell colSpan={5} className="text-center text-muted-foreground py-8">
                      No groups yet. Create a group to start routing models.
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
