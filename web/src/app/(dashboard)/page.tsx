"use client";

import { useEffect, useState } from "react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { api } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import {
  Activity,
  DollarSign,
  TrendingUp,
  Clock,
} from "lucide-react";
import {
  AreaChart,
  Area,
  BarChart,
  Bar,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  PieChart,
  Pie,
  Cell,
} from "recharts";

interface StatsOverview {
  channels: number;
  groups: number;
  api_keys: number;
  requests_today: number;
  tokens_today: number;
  input_tokens: number;
  output_tokens: number;
  cost_today: number;
  failed_today: number;
  success_rate: number;
  avg_latency_ms: number;
}

interface TimeStats {
  date?: string;
  hour?: number;
  input_tokens: number;
  output_tokens: number;
  input_cost: number;
  output_cost: number;
  total_requests: number;
  failed_requests: number;
  total_latency_ms: number;
}

interface ModelStats {
  model_name: string;
  channel_id: number;
  input_tokens: number;
  output_tokens: number;
  total_requests: number;
  input_cost: number;
  output_cost: number;
}

type Period = "hourly" | "daily";

const COLORS = [
  "hsl(220 70% 50%)",
  "hsl(160 60% 45%)",
  "hsl(30 80% 55%)",
  "hsl(280 65% 60%)",
  "hsl(340 75% 55%)",
];

export default function DashboardPage() {
  const { t } = useI18n();
  const [stats, setStats] = useState<StatsOverview | null>(null);
  const [timeData, setTimeData] = useState<TimeStats[]>([]);
  const [models, setModels] = useState<ModelStats[]>([]);
  const [loading, setLoading] = useState(true);
  const [period, setPeriod] = useState<Period>("hourly");

  const fetchData = (p: Period) => {
    setLoading(true);
    const timeEndpoint = p === "hourly" ? "/api/stats/hourly" : "/api/stats/daily";
    const timeParams = p === "daily" ? { params: { days: "7" } } : undefined;
    Promise.all([
      api<StatsOverview>("/api/stats/overview"),
      api<TimeStats[]>(timeEndpoint, timeParams),
      api<ModelStats[]>("/api/stats/models"),
    ])
      .then(([overview, td, modelData]) => {
        setStats(overview);
        setTimeData(td);
        setModels(modelData);
      })
      .catch(() => {})
      .finally(() => setLoading(false));
  };

  useEffect(() => {
    fetchData(period);
  }, [period]);

  const formatCost = (v: number) => {
    if (v >= 1) return `$${v.toFixed(2)}`;
    if (v >= 0.01) return `$${v.toFixed(3)}`;
    return `$${v.toFixed(4)}`;
  };

  const formatTokens = (v: number) => {
    if (v >= 1_000_000) return `${(v / 1_000_000).toFixed(1)}M`;
    if (v >= 1_000) return `${(v / 1_000).toFixed(1)}K`;
    return String(v);
  };

  // Prepare chart data based on period
  const chartData = timeData.map((d) => ({
    label: period === "hourly" ? `${String(d.hour ?? 0).padStart(2, "0")}:00` : (d.date ?? "").slice(5),
    requests: d.total_requests,
    failed: d.failed_requests,
    cost: d.input_cost + d.output_cost,
    inputTokens: d.input_tokens,
    outputTokens: d.output_tokens,
    tokens: d.input_tokens + d.output_tokens,
    avgLatency: d.total_requests > 0 ? Math.round(d.total_latency_ms / d.total_requests) : 0,
  }));

  // Prepare model pie chart data
  const modelPie = models.slice(0, 5).map((m) => ({
    name: m.model_name,
    value: m.total_requests,
  }));

  const periodLabel = period === "hourly" ? t("dashboard.today") : t("dashboard.sevenDays");

  const overviewCards = [
    {
      title: t("dashboard.requestsToday"),
      value: stats?.requests_today ?? 0,
      icon: Activity,
      description: t("dashboard.failedCount", { count: stats?.failed_today ?? 0 }),
      color: "text-blue-500",
    },
    {
      title: t("dashboard.successRate"),
      value: stats ? `${stats.success_rate.toFixed(1)}%` : "-",
      icon: TrendingUp,
      description: t("dashboard.today"),
      color: (stats?.success_rate ?? 100) >= 99 ? "text-green-500" : "text-yellow-500",
    },
    {
      title: t("dashboard.avgLatency"),
      value: stats ? `${stats.avg_latency_ms}ms` : "-",
      icon: Clock,
      description: t("dashboard.perRequestToday"),
      color: "text-purple-500",
    },
    {
      title: t("dashboard.costToday"),
      value: stats ? formatCost(stats.cost_today) : "-",
      icon: DollarSign,
      description: t("dashboard.tokens", { count: formatTokens(stats?.tokens_today ?? 0) }),
      color: "text-emerald-500",
    },
  ];

  const tooltipStyle = {
    backgroundColor: "hsl(var(--popover))",
    border: "1px solid hsl(var(--border))",
    borderRadius: "6px",
    fontSize: "12px",
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">{t("nav.dashboard")}</h1>
          <p className="text-muted-foreground">{t("dashboard.subtitle")}</p>
        </div>
        <div className="flex gap-1 bg-muted rounded-md p-0.5">
          <Button
            variant={period === "hourly" ? "default" : "ghost"}
            size="sm"
            className="h-7 text-xs px-3"
            onClick={() => setPeriod("hourly")}
          >
            {t("dashboard.hourly")}
          </Button>
          <Button
            variant={period === "daily" ? "default" : "ghost"}
            size="sm"
            className="h-7 text-xs px-3"
            onClick={() => setPeriod("daily")}
          >
            {t("dashboard.daily")}
          </Button>
        </div>
      </div>

      {/* Overview Cards */}
      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
        {overviewCards.map((card) => (
          <Card key={card.title}>
            <CardHeader className="flex flex-row items-center justify-between pb-2">
              <CardTitle className="text-sm font-medium text-muted-foreground">
                {card.title}
              </CardTitle>
              <card.icon className={`h-4 w-4 ${card.color}`} />
            </CardHeader>
            <CardContent>
              {loading ? (
                <div className="h-8 w-20 rounded bg-muted animate-pulse" />
              ) : (
                <>
                  <div className="text-2xl font-bold">{card.value}</div>
                  <p className="text-xs text-muted-foreground mt-1">{card.description}</p>
                </>
              )}
            </CardContent>
          </Card>
        ))}
      </div>

      {/* Charts Row 1: Request Trend + Cost Trend */}
      <div className="grid gap-4 md:grid-cols-2">
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-base">{t("dashboard.requests", { period: periodLabel })}</CardTitle>
          </CardHeader>
          <CardContent>
            {loading ? (
              <div className="h-[200px] rounded bg-muted animate-pulse" />
            ) : (
              <ResponsiveContainer width="100%" height={200}>
                <AreaChart data={chartData}>
                  <CartesianGrid strokeDasharray="3 3" className="stroke-muted" />
                  <XAxis dataKey="label" className="text-xs" tick={{ fontSize: 11 }} />
                  <YAxis className="text-xs" tick={{ fontSize: 11 }} />
                  <Tooltip contentStyle={tooltipStyle} />
                  <Area
                    type="monotone"
                    dataKey="requests"
                    stroke="hsl(220 70% 50%)"
                    fill="hsl(220 70% 50% / 0.1)"
                    name={t("dashboard.total")}
                  />
                  <Area
                    type="monotone"
                    dataKey="failed"
                    stroke="hsl(0 72% 51%)"
                    fill="hsl(0 72% 51% / 0.1)"
                    name={t("dashboard.failed")}
                  />
                </AreaChart>
              </ResponsiveContainer>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-base">{t("dashboard.cost", { period: periodLabel })}</CardTitle>
          </CardHeader>
          <CardContent>
            {loading ? (
              <div className="h-[200px] rounded bg-muted animate-pulse" />
            ) : (
              <ResponsiveContainer width="100%" height={200}>
                <BarChart data={chartData}>
                  <CartesianGrid strokeDasharray="3 3" className="stroke-muted" />
                  <XAxis dataKey="label" className="text-xs" tick={{ fontSize: 11 }} />
                  <YAxis className="text-xs" tick={{ fontSize: 11 }} tickFormatter={(v) => `$${v}`} />
                  <Tooltip contentStyle={tooltipStyle} formatter={(value) => [`$${Number(value).toFixed(4)}`, t("logs.cost")]} />
                  <Bar dataKey="cost" fill="hsl(160 60% 45%)" radius={[4, 4, 0, 0]} name={t("logs.cost")} />
                </BarChart>
              </ResponsiveContainer>
            )}
          </CardContent>
        </Card>
      </div>

      {/* Charts Row 2: Token Usage + Model Distribution */}
      <div className="grid gap-4 md:grid-cols-2">
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-base">{t("dashboard.tokenUsage", { period: periodLabel })}</CardTitle>
          </CardHeader>
          <CardContent>
            {loading ? (
              <div className="h-[200px] rounded bg-muted animate-pulse" />
            ) : (
              <ResponsiveContainer width="100%" height={200}>
                <BarChart data={chartData}>
                  <CartesianGrid strokeDasharray="3 3" className="stroke-muted" />
                  <XAxis dataKey="label" className="text-xs" tick={{ fontSize: 11 }} />
                  <YAxis className="text-xs" tick={{ fontSize: 11 }} tickFormatter={formatTokens} />
                  <Tooltip contentStyle={tooltipStyle} formatter={(value) => [formatTokens(Number(value)), ""]} />
                  <Bar dataKey="inputTokens" stackId="tokens" fill="hsl(220 70% 50%)" name={t("dashboard.input")} />
                  <Bar dataKey="outputTokens" stackId="tokens" fill="hsl(280 65% 60%)" radius={[4, 4, 0, 0]} name={t("dashboard.output")} />
                </BarChart>
              </ResponsiveContainer>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-base">{t("dashboard.modelDistribution")}</CardTitle>
          </CardHeader>
          <CardContent>
            {loading || modelPie.length === 0 ? (
              <div className="h-[200px] flex items-center justify-center text-sm text-muted-foreground">
                {loading ? (
                  <div className="h-[200px] w-full rounded bg-muted animate-pulse" />
                ) : (
                  t("dashboard.noData")
                )}
              </div>
            ) : (
              <ResponsiveContainer width="100%" height={200}>
                <PieChart>
                  <Pie
                    data={modelPie}
                    cx="50%"
                    cy="50%"
                    innerRadius={50}
                    outerRadius={80}
                    dataKey="value"
                    label={({ name, percent }) => `${name} ${((percent ?? 0) * 100).toFixed(0)}%`}
                    labelLine={false}
                  >
                    {modelPie.map((_, index) => (
                      <Cell key={`cell-${index}`} fill={COLORS[index % COLORS.length]} />
                    ))}
                  </Pie>
                  <Tooltip contentStyle={tooltipStyle} />
                </PieChart>
              </ResponsiveContainer>
            )}
          </CardContent>
        </Card>
      </div>

      {/* Latency Trend */}
      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-base">{t("dashboard.avgLatencyWithPeriod", { period: periodLabel })}</CardTitle>
        </CardHeader>
        <CardContent>
          {loading ? (
            <div className="h-[160px] rounded bg-muted animate-pulse" />
          ) : (
            <ResponsiveContainer width="100%" height={160}>
              <AreaChart data={chartData}>
                <CartesianGrid strokeDasharray="3 3" className="stroke-muted" />
                <XAxis dataKey="label" className="text-xs" tick={{ fontSize: 11 }} />
                <YAxis className="text-xs" tick={{ fontSize: 11 }} tickFormatter={(v) => `${v}ms`} />
                <Tooltip contentStyle={tooltipStyle} formatter={(value) => [`${value}ms`, t("dashboard.avgLatency")]} />
                <Area
                  type="monotone"
                  dataKey="avgLatency"
                  stroke="hsl(30 80% 55%)"
                  fill="hsl(30 80% 55% / 0.1)"
                  name={t("dashboard.avgLatency")}
                />
              </AreaChart>
            </ResponsiveContainer>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
