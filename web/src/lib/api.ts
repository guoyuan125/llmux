const BASE_URL = process.env.NEXT_PUBLIC_API_URL || "";

export interface FetchOptions extends RequestInit {
  params?: Record<string, string>;
}

function getToken(): string | null {
  if (typeof window === "undefined") return null;
  return localStorage.getItem("llmux_token");
}

export async function api<T = unknown>(path: string, options: FetchOptions = {}): Promise<T> {
  const { params, headers: customHeaders, ...rest } = options;

  let url = `${BASE_URL}${path}`;
  if (params) {
    const search = new URLSearchParams(params);
    url += `?${search.toString()}`;
  }

  const token = getToken();
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    ...(customHeaders as Record<string, string>),
  };
  if (token) {
    headers["Authorization"] = `Bearer ${token}`;
  }

  const res = await fetch(url, { headers, ...rest });

  if (res.status === 401) {
    if (typeof window !== "undefined") {
      localStorage.removeItem("llmux_token");
      window.location.href = "/login";
    }
    throw new Error("Unauthorized");
  }

  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error(body.error || `Request failed: ${res.status}`);
  }

  return res.json();
}

export function setToken(token: string) {
  localStorage.setItem("llmux_token", token);
}

export function clearToken() {
  localStorage.removeItem("llmux_token");
}

export function isAuthenticated(): boolean {
  return !!getToken();
}
