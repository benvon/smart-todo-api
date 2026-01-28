---
name: Todo edit focus and E2E tests
overview: Fix two causes of focus loss when editing todo cards (3s polling re-render and add-tag innerHTML reset). Use a manual checklist for verification; automated E2E deferred until OIDC can be bypassed or mocked for testing.
todos: []
isProject: false
---

# Todo Edit Focus Fix and Manual Testing

## Root causes

Two separate issues cause "refresh under me" and focus loss in the tag input:

### 1. Polling re-render destroys focus

- A **3-second interval** calls `loadTodos()` → `renderTodos()` ([app.js L83–85](web/js/app.js)).
- `renderTodos()` **clears each todo list** with `innerHTML = ''` ([L465–468](web/js/app.js)), then re-appends nodes.
- Edit-mode todos are **preserved by reference** and re-appended ([L454–461, 496–500](web/js/app.js)), but they live *inside* those lists. Clearing the list **removes** the edit card (and the focused "Add tag" input) from the DOM, which **destroys focus**. Re-appending the card does not restore it.

So every ~3 seconds while editing, the form is briefly torn down and put back, and the user loses focus.

### 2. Add-tag flow nukes the input on each Enter

- When the user presses Enter in "Add tag", the handler calls `renderTagsEditor(tagsDiv, ...)` ([L981–989](web/js/app.js)).
- `renderTagsEditor` does `container.innerHTML = ''` ([L1045](web/js/app.js)), which **removes** the add-tag input from the DOM, then rebuilds chips and re-appends the same input. Focus is lost and **not restored** ([L1046–1058](web/js/app.js)).

So adding a tag alone (even without polling) causes focus loss. Users have to click back into the input after every tag.

---

## Fixes

### Fix 1: Skip todo refresh while any card is in edit mode

**File:** [web/js/app.js](web/js/app.js)

- In the 3s `setInterval` callback (L83–85), **skip** `loadTodos()` when `document.querySelector('.todo-edit-mode')` exists.
- Effect: No re-render (and thus no focus-destroying clear/re-append) while the user is editing. Refresh resumes on the next tick after they Save or Cancel.

**Alternative considered:** Restore focus after re-append. That would require tracking the focused element and refocusing it, and the DOM would still flicker. Skipping refresh during edit is simpler and matches user expectation ("don't refresh under me").

### Fix 2: Preserve focus in the tags editor

**File:** [web/js/app.js](web/js/app.js)

**Option A (minimal):** After `renderTagsEditor` appends `addTagInput`, call `addTagInput.focus()`. The input is still removed and re-added (possible brief blink), but focus is restored so the user can immediately type the next tag.

**Option B (cleaner):** Avoid clearing the add-tag input. Refactor so that:

- The "Add tag" input lives in a **separate container** (or stays outside the chips container).
- Only the **chips container** is cleared and rebuilt (e.g. clear chips, append new chips); the input is never removed.
- No focus loss, no refocus logic.

Recommendation: **Option B** if the refactor is straightforward; otherwise **Option A** as a quick fix. Option B is more robust and avoids any blink.

---

## Testing (manual for now)

The project has no E2E or browser-based tests. The default config uses a **real OIDC provider**; there is no clean way to bypass auth for testing today. **Automated E2E (e.g. Playwright) is deferred** until OIDC can be mocked or bypassed.

### Manual checklist

Add a **"Todo edit & tags"** section to [docs/TESTING.md](docs/TESTING.md) (or CONTRIBUTING) so these flows are routinely verified:

- **Add-tag focus:** Edit a todo, focus "Add tag", type a tag, press Enter. Confirm the input *keeps focus* and you can add another tag without clicking back in.
- **No refresh under edit:** Edit a todo, focus "Add tag" (or another field), wait **≥5 seconds**. Confirm you stay in edit mode, keep focus, and can keep typing.
- **Optional:** After Save, confirm the list refreshes (e.g. processing status) so we don't regress refresh when *not* editing.

Run against the stack you already use (e.g. [docker-compose.yml](docker-compose.yml)).

### Deferred: automated E2E

Once OIDC can be bypassed or mocked (test provider, env flag, etc.), add Playwright E2E. Use the root [docker-compose.yml](docker-compose.yml) as the backend: `docker-compose up -d` → `baseURL: http://localhost:3000` → run Playwright from `web/`. Cover the add-tag focus and no-refresh-during-edit scenarios above so these usability issues are caught by CI.

---

## Implementation order

1. **Fix 1:** Skip `loadTodos` during edit mode (small, localized change).
2. **Fix 2:** Refactor tags editor to preserve add-tag input (Option B) or add `addTagInput.focus()` after re-append (Option A).
3. **Docs:** Add a "Todo edit & tags" manual checklist to [docs/TESTING.md](docs/TESTING.md) (add-tag focus, no refresh under edit, optional Save/refresh check).

---

## Summary


| Issue                                         | Cause                                                                   | Fix                                                                                             |
| --------------------------------------------- | ----------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------- |
| "Refresh under me" / lose focus while editing | 3s `loadTodos` → `renderTodos` clears lists, removes edit card from DOM | Skip refresh when `.todo-edit-mode` exists                                                      |
| Lose focus when adding a tag                  | `renderTagsEditor` uses `innerHTML = ''`, nukes add-tag input           | Keep input in separate container (Option B) or `addTagInput.focus()` after re-append (Option A) |
| Verification                                  | No E2E (OIDC blocks automation)                                         | Manual checklist in TESTING.md; automated E2E deferred until auth can be bypassed/mocked        |


These changes fix the behavior you're seeing. A manual checklist keeps edit/tag flows consistently verified; Playwright E2E can be added later when OIDC is testable.