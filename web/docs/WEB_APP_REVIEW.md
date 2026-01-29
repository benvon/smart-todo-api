# Web App Review Notes

## Config and API

- **Config:** Config is loaded in `config.js` and exposes `window.CONFIG_LOADED` (a promise) and `window.API_BASE_URL`. The API layer centralizes "ensure config" in `apiRequest()`: every API call (via `apiRequest`) awaits `CONFIG_LOADED` before running, so individual functions like `getCurrentUser`, `getAIContext`, and `getTagStats` do not need to check config themselves.
- **API surface:** All API functions are consumed via ES module imports. The api module exports only; there are no `window.*` assignments for API functions.

## Profile

- **Load path:** The profile panel has a single load path: on open, it shows loading, then fetches user, AI context, and tag stats in parallel with a 10s timeout (`PROFILE_FETCH_TIMEOUT_MS`). A hanging request will not leave the UI stuck on "Loading..."; timeout and API errors are surfaced via `showError`.
- **Context:** Profile and chat share a single source of truth for AI context via `context.js` (`getContext` / `setContext`). Saving context from the profile updates the shared context so chat stays in sync.

## Module Structure

- **app.js:** Bootstrap, event wiring, API status indicator/dropdown, and add-todo handler. Imports `loadTodos` from `todo-list.js` for refresh and after add.
- **todo-list.js:** Todo list state, rendering (using the todo card builder), drag-drop, and all todo actions (complete, delete, edit, reprocess, edit due date). Exports `loadTodos`.
- **panels/panel.js:** Minimal panel helper (show/hide, setContent with loading/error/content). No external dependency.
- **cards/todo-card.js:** Builds a single todo card DOM node given a todo and handlers.
- **cards/profile-content.js:** Builds the profile panel body content (user info, context textarea, tag stats, logout) given profile data and handlers.
- **context.js:** Single source of truth for current AI context text; used by chat and profile.
