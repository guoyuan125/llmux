"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import {
  LayoutDashboard,
  Radio,
  Layers,
  KeyRound,
  ScrollText,
  Settings,
  LogOut,
  Zap,
  Languages,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { clearToken } from "@/lib/api";
import { useI18n } from "@/lib/i18n";

const navItems = [
  { href: "/", labelKey: "nav.dashboard", icon: LayoutDashboard },
  { href: "/channels", labelKey: "common.channels", icon: Radio },
  { href: "/groups", labelKey: "nav.groups", icon: Layers },
  { href: "/apikeys", labelKey: "nav.apiKeys", icon: KeyRound },
  { href: "/logs", labelKey: "nav.logs", icon: ScrollText },
  { href: "/settings", labelKey: "nav.settings", icon: Settings },
] as const;

export function AppSidebar() {
  const pathname = usePathname();
  const { locale, setLocale, t } = useI18n();

  const handleLogout = () => {
    clearToken();
    window.location.href = "/login";
  };

  return (
    <aside className="hidden md:flex md:w-60 md:flex-col md:fixed md:inset-y-0 border-r border-border bg-sidebar">
      <div className="flex h-14 items-center gap-2 px-4 border-b border-border">
        <Zap className="h-5 w-5 text-primary" />
        <span className="font-semibold text-lg">llmux</span>
      </div>

      <nav className="flex-1 flex flex-col gap-1 p-3">
        {navItems.map((item) => {
          const active =
            item.href === "/"
              ? pathname === "/"
              : pathname.startsWith(item.href);
          return (
            <Link
              key={item.href}
              href={item.href}
              className={cn(
                "flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors",
                active
                  ? "bg-sidebar-accent text-sidebar-accent-foreground"
                  : "text-sidebar-foreground/70 hover:bg-sidebar-accent/50 hover:text-sidebar-foreground"
              )}
            >
              <item.icon className="h-4 w-4" />
              {t(item.labelKey)}
            </Link>
          );
        })}
      </nav>

      <div className="space-y-2 p-3 border-t border-border">
        <button
          type="button"
          onClick={() => setLocale(locale === "zh" ? "en" : "zh")}
          className="flex w-full items-center justify-between rounded-md px-3 py-2 text-sm font-medium text-sidebar-foreground/70 hover:bg-sidebar-accent/50 hover:text-sidebar-foreground transition-colors"
        >
          <span className="flex items-center gap-3">
            <Languages className="h-4 w-4" />
            {t("common.language")}
          </span>
          <span className="text-xs">{locale === "zh" ? "中文" : "EN"}</span>
        </button>
        <button
          onClick={handleLogout}
          className="flex w-full items-center gap-3 rounded-md px-3 py-2 text-sm font-medium text-sidebar-foreground/70 hover:bg-sidebar-accent/50 hover:text-sidebar-foreground transition-colors"
        >
          <LogOut className="h-4 w-4" />
          {t("nav.logout")}
        </button>
      </div>
    </aside>
  );
}
