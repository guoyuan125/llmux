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
      toast.success("API key created");
      setDialogOpen(false);
      setForm({ name: "", rpm_limit: 60, tpm_limit: 100000, max_cost: 0, supported_models: "" });
      fetchKeys();
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : "Failed");
    }
  };

  const handleDelete = async (id: number) => {
    if (!confirm("Delete this API key?")) return;
    try {
      await api(`/api/apikeys/${id}`, { method: "DELETE" });
      toast.success("API key deleted");
      fetchKeys();
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : "Failed");
    }
  };

  const copyKey = (key: string) => {
    navigator.clipboard.writeText(key);
    toast.success("Copied to clipboard");
  };

  const maskKey = (key: string) => {
    if (key.length <= 8) return "****";
    return key.slice(0, 4) + "..." + key.slice(-4);
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">API Keys</h1>
          <p className="text-muted-foreground">Manage client API keys for accessing the gateway</p>
        </div>
        <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
          <DialogTrigger render={<Button />}>
            <Plus className="h-4 w-4 mr-2" />Create Key
          </DialogTrigger>
          <DialogContent className="max-w-lg">
            <DialogHeader>
              <DialogTitle>Create API Key</DialogTitle>
            </DialogHeader>
            <div className="grid gap-4 py-4">
              <div className="grid gap-2">
                <Label>Name</Label>
                <Input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} placeholder="my-app" />
              </div>
              <div className="grid grid-cols-2 gap-4">
                <div className="grid gap-2">
                  <Label>RPM Limit</Label>
                  <Input type="number" value={form.rpm_limit} onChange={(e) => setForm({ ...form, rpm_limit: parseInt(e.target.value) || 0 })} />
                </div>
                <div className="grid gap-2">
                  <Label>TPM Limit</Label>
                  <Input type="number" value={form.tpm_limit} onChange={(e) => setForm({ ...form, tpm_limit: parseInt(e.target.value) || 0 })} />
                </div>
              </div>
              <div className="grid gap-2">
                <Label>Allowed Models (comma-separated, empty = all)</Label>
                <Input value={form.supported_models} onChange={(e) => setForm({ ...form, supported_models: e.target.value })} placeholder="gpt-4o,claude-sonnet-4-20250514" />
              </div>
              <Button onClick={handleCreate}>Create</Button>
            </div>
          </DialogContent>
        </Dialog>
      </div>

      <Card>
        <CardHeader><CardTitle>All API Keys</CardTitle></CardHeader>
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
                  <TableHead>Key</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>RPM / TPM</TableHead>
                  <TableHead>Models</TableHead>
                  <TableHead className="text-right">Actions</TableHead>
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
                        {k.enabled ? "Active" : "Disabled"}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-muted-foreground text-xs">
                      {k.rpm_limit} / {k.tpm_limit}
                    </TableCell>
                    <TableCell className="text-muted-foreground text-xs max-w-32 truncate">
                      {k.supported_models || "All"}
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
                      No API keys yet. Create one to allow clients to use the gateway.
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
