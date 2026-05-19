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
import { Plus, Pencil, Trash2 } from "lucide-react";
import { toast } from "sonner";

interface GroupItem {
  channel_id: number;
  model_name: string;
  priority: number;
  weight: number;
}

interface Group {
  id: number;
  name: string;
  mode: string;
  match_regex: string;
  session_keep_time: number;
  first_token_timeout: number;
  items: GroupItem[];
}

const modes = [
  { value: "round_robin", label: "Round Robin" },
  { value: "random", label: "Random" },
  { value: "failover", label: "Failover (Priority)" },
  { value: "weighted", label: "Weighted" },
  { value: "least_cost", label: "Least Cost" },
  { value: "least_latency", label: "Least Latency" },
];

export default function GroupsPage() {
  const [groups, setGroups] = useState<Group[]>([]);
  const [loading, setLoading] = useState(true);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editing, setEditing] = useState<Group | null>(null);

  const [form, setForm] = useState({
    name: "",
    mode: "round_robin",
    match_regex: "",
    session_keep_time: 0,
    first_token_timeout: 0,
  });

  const fetchGroups = () => {
    api<Group[]>("/api/groups")
      .then(setGroups)
      .catch(() => {})
      .finally(() => setLoading(false));
  };

  useEffect(() => { fetchGroups(); }, []);

  const resetForm = () => {
    setForm({ name: "", mode: "round_robin", match_regex: "", session_keep_time: 0, first_token_timeout: 0 });
    setEditing(null);
  };

  const handleSubmit = async () => {
    try {
      const payload = { ...form };
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
      mode: g.mode,
      match_regex: g.match_regex || "",
      session_keep_time: g.session_keep_time || 0,
      first_token_timeout: g.first_token_timeout || 0,
    });
    setDialogOpen(true);
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">Groups</h1>
          <p className="text-muted-foreground">Model routing groups with load balancing</p>
        </div>
        <Dialog open={dialogOpen} onOpenChange={(open) => { setDialogOpen(open); if (!open) resetForm(); }}>
          <DialogTrigger render={<Button />}>
            <Plus className="h-4 w-4 mr-2" />Add Group
          </DialogTrigger>
          <DialogContent className="max-w-lg">
            <DialogHeader>
              <DialogTitle>{editing ? "Edit Group" : "New Group"}</DialogTitle>
            </DialogHeader>
            <div className="grid gap-4 py-4">
              <div className="grid gap-2">
                <Label>Name (model name clients request)</Label>
                <Input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} placeholder="gpt-4o" />
              </div>
              <div className="grid gap-2">
                <Label>Balancing Mode</Label>
                <select
                  className="flex h-8 w-full rounded-lg border border-input bg-transparent px-3 py-1 text-sm transition-colors focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
                  value={form.mode}
                  onChange={(e) => setForm({ ...form, mode: e.target.value })}
                >
                  {modes.map((m) => (
                    <option key={m.value} value={m.value}>{m.label}</option>
                  ))}
                </select>
              </div>
              <div className="grid gap-2">
                <Label>Match Regex (optional, for wildcard routing)</Label>
                <Input value={form.match_regex} onChange={(e) => setForm({ ...form, match_regex: e.target.value })} placeholder="gpt-4*" />
              </div>
              <div className="grid grid-cols-2 gap-4">
                <div className="grid gap-2">
                  <Label>Session Keep (sec)</Label>
                  <Input type="number" value={form.session_keep_time} onChange={(e) => setForm({ ...form, session_keep_time: parseInt(e.target.value) || 0 })} />
                </div>
                <div className="grid gap-2">
                  <Label>First Token Timeout (sec)</Label>
                  <Input type="number" value={form.first_token_timeout} onChange={(e) => setForm({ ...form, first_token_timeout: parseInt(e.target.value) || 0 })} />
                </div>
              </div>
              <Button onClick={handleSubmit}>{editing ? "Update" : "Create"}</Button>
            </div>
          </DialogContent>
        </Dialog>
      </div>

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
                  <TableHead>Name</TableHead>
                  <TableHead>Mode</TableHead>
                  <TableHead>Regex</TableHead>
                  <TableHead>Channels</TableHead>
                  <TableHead className="text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {groups.map((g) => (
                  <TableRow key={g.id}>
                    <TableCell className="font-medium">{g.name}</TableCell>
                    <TableCell><Badge variant="secondary">{g.mode}</Badge></TableCell>
                    <TableCell className="text-muted-foreground text-xs">{g.match_regex || "-"}</TableCell>
                    <TableCell>{g.items?.length || 0}</TableCell>
                    <TableCell className="text-right">
                      <Button variant="ghost" size="icon" onClick={() => handleEdit(g)}><Pencil className="h-4 w-4" /></Button>
                      <Button variant="ghost" size="icon" onClick={() => handleDelete(g.id)}><Trash2 className="h-4 w-4 text-destructive" /></Button>
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
