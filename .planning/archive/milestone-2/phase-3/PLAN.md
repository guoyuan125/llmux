# Phase 3: Logs 页面增强 — PLAN

**Goal:** error 日志行突出显示，展开可查看格式化请求/响应全文。

---

## Context

### 前端现状

`web/src/app/(dashboard)/logs/page.tsx` 中已有：

- ✅ `expandedId` state + `toggleExpand` 函数（line 44, 89-91）
- ✅ ChevronDown / ChevronRight 展开箭头（line 151-154）
- ✅ 展开行显示格式化 JSON request_body / response_body（line 183-254）
- ✅ `max-h-48 overflow-auto` 滚动限制（line 239, 247）
- ❌ **缺失**：error 行整行红色背景 —— line 147 `className="cursor-pointer hover:bg-muted/50"` 对 error/normal 行一视同仁

---

## Tasks

### Task 1 — 前端：error 行添加红色背景

**文件：** `web/src/app/(dashboard)/logs/page.tsx`

**定位：** line 145-149，`<TableRow>` 的 className

**当前：**
```tsx
<TableRow
  key={log.id}
  className="cursor-pointer hover:bg-muted/50"
  onClick={() => toggleExpand(log.id)}
>
```

**替换为：**
```tsx
<TableRow
  key={log.id}
  className={`cursor-pointer ${log.error ? "bg-red-50 dark:bg-red-950/20 hover:bg-red-100 dark:hover:bg-red-950/30" : "hover:bg-muted/50"}`}
  onClick={() => toggleExpand(log.id)}
>
```

---

## Constraints

- 只改一处 className，不动其他逻辑
- 使用 Tailwind 内置颜色，不引入新 CSS

---

## Verification

1. 启动服务
2. 发送一次会失败的请求（错误 API key 或无效 endpoint）
3. Logs 页面该行整行显示红色背景
4. 正常请求行保持原有 hover:bg-muted/50 样式
5. 点击展开 error 行，确认 request_body / response_body 可见且可滚动
