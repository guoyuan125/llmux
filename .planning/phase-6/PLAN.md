---
phase: "6-channel-model-management-ui"
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - web/src/app/(dashboard)/channels/page.tsx
autonomous: true
requirements:
  - CHN-01
  - CHN-02
must_haves:
  truths:
    - "Channel list table shows a Models column with badge pills for each channel's configured models"
    - "Channels with no configured models display 'No models' placeholder text in that column"
    - "Edit channel dialog has a Models section showing current custom_models as removable pills"
    - "User can type a model name into an input and click Add to append it to the list"
    - "User can click × on a pill to remove that model from the list"
    - "Saving the dialog persists custom_models; a full page reload shows the change"
    - "The models badge column shows custom_models ∪ models (union), deduplicated"
  artifacts:
    - path: "web/src/app/(dashboard)/channels/page.tsx"
      provides: "Channel list + edit dialog with full model management"
      contains: "custom_models"
  key_links:
    - from: "EditChannelDialog form state (customModels: string[])"
      to: "PUT /api/channels/:id payload (custom_models: comma-joined string)"
      pattern: "customModels.join(',')"
    - from: "Channel list table row"
      to: "models badge column"
      pattern: "mergedModels(ch)"
---

<objective>
Implement CHN-01 and CHN-02: display the merged model set inline in the Channel table, and
add a model add/remove UI to the edit dialog.

Purpose: Administrators need visibility into each channel's configured models and must be able
to manually curate that list without leaving the UI.

Output: Updated web/src/app/(dashboard)/channels/page.tsx — no new files, no new backend endpoints.
</objective>

<execution_context>
This is a pure frontend task. The backend already supports custom_models:
- GET /api/channels returns { models: string, custom_models: string } on every Channel object.
- PUT /api/channels/:id accepts custom_models in the request body (handler/channel.go line 56).

No backend changes are needed.
</execution_context>

<context>
@web/src/app/(dashboard)/channels/page.tsx
@web/src/app/(dashboard)/groups/page.tsx
@internal/model/channel.go
</context>

<interfaces>
<!-- Key types extracted from the current codebase. -->

From internal/model/channel.go:
  Channel.Models        string  // json:"models"        — comma-separated, upstream-synced, read-only
  Channel.CustomModels  string  // json:"custom_models" — user-defined, writable

PUT /api/channels/:id — UpdateChannel in handler/channel.go line 51:
  Reads input.CustomModels and writes it to "custom_models" DB column.
  The existing payload shape in page.tsx handleSubmit() must be extended with:
    custom_models: form.customModels.join(",")

shadcn/ui components already imported in channels/page.tsx: Badge, Input, Label, Button.
lucide-react icons already imported: Plus, Pencil, Trash2.
Need to add: X (already imported in groups/page.tsx, add to channels/page.tsx import).
</interfaces>

<tasks>

<task type="auto">
  <name>Task 1: Extend Channel interface and add Models column to table</name>
  <files>web/src/app/(dashboard)/channels/page.tsx</files>
  <action>
Make the following changes to page.tsx:

1. Add X to the lucide-react import (line 16):
   Change: import { Plus, Pencil, Trash2 } from "lucide-react";
   To:     import { Plus, Pencil, Trash2, X } from "lucide-react";

2. Extend the Channel interface (lines 19-28) to add the two model fields:
   Add after the proxy field:
     models: string;
     custom_models: string;

3. Add a helper function after the channelTypes constant that computes the deduplicated
   union of custom_models and models for display. Place it before the ChannelsPage function:

     function mergedModels(ch: Channel): string[] {
       const parts = [
         ...(ch.custom_models || "").split(","),
         ...(ch.models || "").split(","),
       ]
         .map((s) => s.trim())
         .filter(Boolean);
       return Array.from(new Set(parts));
     }

4. In the TableHeader row (line 180), insert a new <TableHead> before the "Actions" head:
   Add after the "Keys" <TableHead>:
     <TableHead>Models</TableHead>

5. In each TableRow (after the Keys cell at line 202), insert a new <TableCell>:
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

6. Update the empty-row colspan from 6 to 7 (line 215):
   Change: colSpan={6}
   To:     colSpan={7}
  </action>
  <verify>
    After `cd /Users/liuguoyuan/workspace/llmux/web && pnpm build` (no type errors),
    start the server and visit /channels — the table must have a Models column.
    A channel with custom_models="gpt-4o,gpt-3.5-turbo" must show two Badge pills.
    A channel with no custom_models must show "No models" text.
  </verify>
  <done>
    Models column visible in channel list. Badges show union of custom_models and models,
    deduplicated. Channels with no models show "No models" placeholder.
  </done>
</task>

<task type="auto">
  <name>Task 2: Add model management section to edit dialog</name>
  <files>web/src/app/(dashboard)/channels/page.tsx</files>
  <action>
Extend the form state and dialog to support adding and removing custom models.

1. Add customModels to the form state (line 42-50).
   Change the form useState initializer from:
     const [form, setForm] = useState({
       name: "",
       type: "openai",
       enabled: true,
       base_url: "",
       key: "",
       proxy: "",
       param_override: "",
     });
   To:
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

   Also add a local state for the add-model input text. Place it after the form useState:
     const [newModel, setNewModel] = useState("");

2. Update resetForm() (lines 61-64) to include customModels and reset newModel:
   Change:
     const resetForm = () => {
       setForm({ name: "", type: "openai", enabled: true, base_url: "", key: "", proxy: "", param_override: "" });
       setEditing(null);
     };
   To:
     const resetForm = () => {
       setForm({ name: "", type: "openai", enabled: true, base_url: "", key: "", proxy: "", param_override: "", customModels: [] });
       setNewModel("");
       setEditing(null);
     };

3. Update handleEdit() (lines 104-116) to populate customModels from the channel's
   custom_models string. Inside handleEdit, change the setForm call to include:
     customModels: ch.custom_models
       ? ch.custom_models.split(",").map((s) => s.trim()).filter(Boolean)
       : [],

4. Update handleSubmit() payload (lines 68-76) to include custom_models:
   Add to the payload object:
     custom_models: form.customModels.join(","),

5. Add two helper callbacks before the return statement (or inline as arrow functions):
   - addModel: trims newModel, skips if empty or already present, appends to customModels, clears newModel.
   - removeModel: filters the model out of customModels by value.

   Place these after resetForm():

     const addModel = () => {
       const m = newModel.trim();
       if (!m || form.customModels.includes(m)) return;
       setForm((prev) => ({ ...prev, customModels: [...prev.customModels, m] }));
       setNewModel("");
     };

     const removeModel = (m: string) => {
       setForm((prev) => ({ ...prev, customModels: prev.customModels.filter((x) => x !== m) }));
     };

6. In the DialogContent (starting at line 129), add a Models section after the
   "Proxy (optional)" grid block and before the Submit Button. Insert:

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

  Note on key placement: the "models" read-only field from the server is NOT shown as an
  editable field. Only custom_models is editable. The merged union display belongs in the
  table (Task 1), not in the dialog. The dialog edits custom_models only — per the design
  decision in STATE.md: "custom_models is the ONLY write target".
  </action>
  <verify>
    After `pnpm build` (no type errors) and server restart:
    1. Open edit dialog for a channel that has no custom_models — Models section shows
       "No models configured" and an add input.
    2. Type "gpt-4o" in the input, press Enter — pill appears, input clears.
    3. Type "gpt-3.5-turbo", click Add button — second pill appears.
    4. Click × on "gpt-4o" — it disappears.
    5. Click Update — PUT /api/channels/:id is called.
    6. Verify via: curl -s http://localhost:9000/api/channels | jq '.[] | {name, custom_models}'
       The saved channel must have custom_models="gpt-3.5-turbo".
    7. Reload the page — channel row in table shows the gpt-3.5-turbo badge.
  </verify>
  <done>
    Edit dialog has a working Models section. Adding a model via Enter or button appends a
    pill. Removing a model via × removes the pill. Saving serializes to custom_models
    comma-string. Reload confirms persistence. Table badge reflects saved value.
  </done>
</task>

<task type="auto">
  <name>Task 3: Build, restart, and run e2e smoke test</name>
  <files>/tmp/test_phase6.py</files>
  <action>
Step A — Build frontend:
  cd /Users/liuguoyuan/workspace/llmux/web && pnpm build
  Must complete without errors. Fix any TypeScript type errors before proceeding.

Step B — Build backend:
  cd /Users/liuguoyuan/workspace/llmux && go build -o llmux .
  Must compile cleanly.

Step C — Restart server:
  Kill existing llmux process if running (check llmux.pid or pkill llmux).
  Start: cd /Users/liuguoyuan/workspace/llmux && ./llmux start &
  Wait for the server to be ready (poll GET /health until 200, max 10 attempts).

Step D — Write and run the Playwright e2e test at /tmp/test_phase6.py.
  The test uses the Python playwright library (sync_api). It must:

  1. Import: from playwright.sync_api import sync_playwright, expect

  2. Test function test_models_column():
     - Navigate to http://localhost:9000/channels
     - Assert a column header with text "Models" is visible in the table.

  3. Test function test_add_remove_model():
     - Navigate to /channels.
     - Click the pencil (edit) icon on the first channel row.
     - Wait for the dialog to appear (look for element with text "Edit Channel").
     - Find the model name input (placeholder "e.g. gpt-4o").
     - Type "test-model-phase6".
     - Press Enter (or click the Add button).
     - Assert a span/badge with text "test-model-phase6" is visible in the dialog.
     - Click the × button on the "test-model-phase6" pill.
     - Assert the pill is no longer visible.
     - Type "persist-model-phase6" and press Enter.
     - Click the "Update" button.
     - Wait for the dialog to close.
     - Assert the channel row now shows a badge with text "persist-model-phase6".

  4. Test function test_no_models_placeholder():
     - If there is a channel with no custom_models and no models, its Models cell
       must contain the text "No models".
     - This test may be skipped if all channels have at least one model.

  Run the test file: python /tmp/test_phase6.py
  All assertions must pass. Record pass/fail output.

  Write the complete test file to /tmp/test_phase6.py using the Write tool before running.
  </action>
  <verify>
    `python /tmp/test_phase6.py` exits 0 with all tests passing.
    `go build -o llmux .` exits 0 in the repo root.
    `pnpm build` exits 0 in the web/ directory.
  </verify>
  <done>
    Frontend built, backend compiled, server running, all three Playwright test functions
    pass without assertion errors.
  </done>
</task>

</tasks>

<verification>
End-to-end verification sequence (run in order after all tasks complete):

1. `cd /Users/liuguoyuan/workspace/llmux/web && pnpm build` — zero errors
2. `cd /Users/liuguoyuan/workspace/llmux && go build -o llmux .` — zero errors
3. Server running at :9000
4. `curl -s http://localhost:9000/api/channels | jq '.[0] | {custom_models, models}'`
   — both fields present in response
5. Manual: Open /channels in browser, confirm Models column exists
6. Manual: Edit a channel, add "gpt-4o-test", save, reload, confirm badge visible
7. `python /tmp/test_phase6.py` — all tests pass
</verification>

<success_criteria>
1. Channel list table has a "Models" column showing Badge pills for custom_models ∪ models (union, deduplicated).
2. Channels with empty custom_models and empty models show "No models" text in that column.
3. Edit dialog has a Models section with: current custom_models rendered as removable pills, add-input + Add button, and on-Enter support.
4. Custom model changes persist through PUT /api/channels/:id and survive a page reload.
5. `pnpm build` and `go build` both succeed with no errors.
6. Playwright tests at /tmp/test_phase6.py all pass.
</success_criteria>

<output>
When all tasks are complete, create:
  .planning/phases/6-channel-model-management-ui/06-01-SUMMARY.md

Include:
- What was changed (file diffs summary)
- Verification results (build output, test results)
- Any deviations from the plan
</output>
