"use client";

import { useEffect, useState, useRef } from "react";
import { api } from "@/lib/api";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from "@/components/ui/table";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Search, RefreshCw, Radio, ChevronDown, ChevronRight } from "lucide-react";

interface AuditLog {
  id: number;
  request_id: string;
  api_key_id: number;
  model: string;
  group_name: string;
  upstream_model: string;
  channel_id: number;
  channel_name: string;
  status_code: number;
  latency_ms: number;
  first_token_ms: number;
  input_tokens: number;
  output_tokens: number;
  cost: number;
  attempts: number;
  stream: boolean;
  error: string;
  request_body: string;
  response_body: string;
  created_at: string;
}

const MAX_LOGS = 200;

export default function LogsPage() {
  const [logs, setLogs] = useState<AuditLog[]>([]);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState("");
  const [connected, setConnected] = useState(false);
  const [expandedId, setExpandedId] = useState<number | null>(null);
  const esRef = useRef<EventSource | null>(null);

  const fetchLogs = () => {
    setLoading(true);
    api<{ total: number; data: AuditLog[] }>("/api/logs", { params: search ? { model: search } : undefined })
      .then((res) => setLogs(res.data || []))
      .catch(() => {})
      .finally(() => setLoading(false));
  };

  useEffect(() => {
    fetchLogs();

    const token = typeof window !== "undefined" ? localStorage.getItem("llmux_token") : null;
    if (!token) return;

    const es = new EventSource(`/api/logs/stream?token=${encodeURIComponent(token)}`);
    esRef.current = es;

    es.addEventListener("log", (e) => {
      const log: AuditLog = JSON.parse(e.data);
      setLogs((prev) => {
        if (search && !log.model.includes(search)) return prev;
        const next = [log, ...prev];
        if (next.length > MAX_LOGS) next.length = MAX_LOGS;
        return next;
      });
    });

    es.onopen = () => setConnected(true);
    es.onerror = () => setConnected(false);

    return () => {
      es.close();
      esRef.current = null;
    };
  }, []);

  const formatTime = (ts: string) => {
    if (!ts) return "-";
    const d = new Date(ts);
    return d.toLocaleTimeString();
  };

  const toggleExpand = (id: number) => {
    setExpandedId(expandedId === id ? null : id);
  };

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">Request Logs</h1>
        <p className="text-muted-foreground">Audit trail of all gateway requests</p>
      </div>

      <div className="flex gap-2 items-center">
        <div className="relative flex-1 max-w-sm">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
          <Input
            className="pl-9"
            placeholder="Filter by model..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && fetchLogs()}
          />
        </div>
        <Button variant="outline" size="icon" onClick={fetchLogs}>
          <RefreshCw className="h-4 w-4" />
        </Button>
        <span className="flex items-center text-xs text-muted-foreground gap-1.5">
          <Radio className={`h-3 w-3 ${connected ? "text-green-500" : "text-red-500"}`} />
          {connected ? "Live" : "Disconnected"}
        </span>
      </div>

      <Card>
        <CardHeader><CardTitle>Recent Requests</CardTitle></CardHeader>
        <CardContent>
          {loading ? (
            <div className="space-y-2">
              {[1, 2, 3, 4, 5].map((i) => <div key={i} className="h-10 rounded bg-muted animate-pulse" />)}
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="w-6"></TableHead>
                  <TableHead>Time</TableHead>
                  <TableHead>Request Model</TableHead>
                  <TableHead>Group</TableHead>
                  <TableHead>Channel → Upstream</TableHead>
                  <TableHead>Latency</TableHead>
                  <TableHead>TTFT</TableHead>
                  <TableHead>Tokens</TableHead>
                  <TableHead>Status</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {logs.map((log) => (
                  <>
                    <TableRow
                      key={log.id}
                      className={`cursor-pointer ${log.error ? "bg-red-50 dark:bg-red-950/20 hover:bg-red-100 dark:hover:bg-red-950/30" : "hover:bg-muted/50"}`}
                      onClick={() => toggleExpand(log.id)}
                    >
                      <TableCell className="w-6 px-2">
                        {expandedId === log.id
                          ? <ChevronDown className="h-3.5 w-3.5 text-muted-foreground" />
                          : <ChevronRight className="h-3.5 w-3.5 text-muted-foreground" />}
                      </TableCell>
                      <TableCell className="text-xs text-muted-foreground whitespace-nowrap">
                        {formatTime(log.created_at)}
                      </TableCell>
                      <TableCell className="font-medium text-sm">{log.model}</TableCell>
                      <TableCell className="text-xs text-muted-foreground">{log.group_name || "-"}</TableCell>
                      <TableCell className="text-sm">
                        {log.channel_name}
                        {log.upstream_model && log.upstream_model !== log.model && (
                          <span className="text-muted-foreground"> → {log.upstream_model}</span>
                        )}
                      </TableCell>
                      <TableCell className="text-xs">{log.latency_ms}ms</TableCell>
                      <TableCell className="text-xs">
                        {log.stream ? `${log.first_token_ms}ms` : "-"}
                      </TableCell>
                      <TableCell className="text-xs">
                        {log.input_tokens + log.output_tokens > 0
                          ? `${log.input_tokens} / ${log.output_tokens}`
                          : "-"}
                      </TableCell>
                      <TableCell>
                        {log.error ? (
                          <Badge variant="destructive" className="text-xs">Error</Badge>
                        ) : (
                          <Badge variant="default" className="text-xs">OK</Badge>
                        )}
                      </TableCell>
                    </TableRow>
                    {expandedId === log.id && (
                      <TableRow key={`${log.id}-detail`} className="bg-muted/30">
                        <TableCell colSpan={9} className="p-4">
                          <div className="grid grid-cols-2 md:grid-cols-4 gap-3 text-xs">
                            <div>
                              <span className="text-muted-foreground">Request ID</span>
                              <p className="font-mono mt-0.5">{log.request_id || "-"}</p>
                            </div>
                            <div>
                              <span className="text-muted-foreground">API Key ID</span>
                              <p className="mt-0.5">{log.api_key_id}</p>
                            </div>
                            <div>
                              <span className="text-muted-foreground">Channel ID</span>
                              <p className="mt-0.5">{log.channel_id}</p>
                            </div>
                            <div>
                              <span className="text-muted-foreground">Attempts</span>
                              <p className="mt-0.5">{log.attempts}</p>
                            </div>
                            <div>
                              <span className="text-muted-foreground">Stream</span>
                              <p className="mt-0.5">{log.stream ? "Yes" : "No"}</p>
                            </div>
                            <div>
                              <span className="text-muted-foreground">Input Tokens</span>
                              <p className="mt-0.5">{log.input_tokens}</p>
                            </div>
                            <div>
                              <span className="text-muted-foreground">Output Tokens</span>
                              <p className="mt-0.5">{log.output_tokens}</p>
                            </div>
                            <div>
                              <span className="text-muted-foreground">Cost</span>
                              <p className="mt-0.5">{log.cost > 0 ? `$${log.cost.toFixed(6)}` : "-"}</p>
                            </div>
                            <div>
                              <span className="text-muted-foreground">Created At</span>
                              <p className="mt-0.5">{new Date(log.created_at).toLocaleString()}</p>
                            </div>
                            <div>
                              <span className="text-muted-foreground">Status Code</span>
                              <p className="mt-0.5">{log.status_code || "-"}</p>
                            </div>
                          </div>
                          {log.error && (
                            <div className="mt-3 p-2 rounded bg-destructive/10 border border-destructive/20">
                              <span className="text-xs text-muted-foreground">Error</span>
                              <p className="text-xs font-mono mt-0.5 text-destructive whitespace-pre-wrap break-all">
                                {log.error}
                              </p>
                            </div>
                          )}
                          {log.request_body && (
                            <div className="mt-3 p-2 rounded bg-muted/50 border border-border">
                              <span className="text-xs text-muted-foreground">Request Body</span>
                              <pre className="text-xs font-mono mt-1 whitespace-pre-wrap break-all max-h-48 overflow-auto">
                                {(() => { try { return JSON.stringify(JSON.parse(log.request_body), null, 2); } catch { return log.request_body; } })()}
                              </pre>
                            </div>
                          )}
                          {log.response_body && log.response_body !== log.error && (
                            <div className="mt-3 p-2 rounded bg-muted/50 border border-border">
                              <span className="text-xs text-muted-foreground">Response Body</span>
                              <pre className="text-xs font-mono mt-1 whitespace-pre-wrap break-all max-h-48 overflow-auto">
                                {log.response_body}
                              </pre>
                            </div>
                          )}
                        </TableCell>
                      </TableRow>
                    )}
                  </>
                ))}
                {logs.length === 0 && (
                  <TableRow>
                    <TableCell colSpan={8} className="text-center text-muted-foreground py-8">
                      No request logs yet.
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
