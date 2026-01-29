# Frontend JavaScript Dependencies

## Build System

The frontend uses a modern build system with npm and esbuild. All JavaScript files are bundled into `dist/` directory.

### Dependencies

**Production Dependencies:**
- `chrono-node` - Natural language date parsing library
- `dayjs` - Lightweight date manipulation and formatting library

**Development Dependencies:**
- `esbuild` - Fast JavaScript bundler
- `eslint` - JavaScript linter and code quality tool
- `@eslint/js` - ESLint JavaScript configuration
- `c8` - Native Node.js test coverage tool
- `semantic-release` - Automated version management and releases
- `@semantic-release/commit-analyzer` - Analyzes commits for semantic versioning
- `@semantic-release/release-notes-generator` - Generates release notes
- `@semantic-release/github` - GitHub release integration

### Building

```bash
# Install dependencies (first time only)
cd web
npm install

# Build for production
npm run build

# Build in watch mode for development
npm run dev

# Run tests
npm test

# Run tests with coverage
npm run test:coverage

# Lint code
npm run lint

# Fix linting issues
npm run lint:fix

# Security audit
npm run security:audit

# Check security and outdated packages
npm run security:check
```

Build output is in `web/dist/`:
- `dist/app.js` - Main application bundle (used by app.html)
- `dist/index.js` - Login page bundle (used by index.html)

### File Dependency Graph

```
app-entry.js
  ├─ config.js (sets window.API_BASE_URL, CONFIG_LOADED)
  ├─ jwt.js (token utilities)
  ├─ dateutils.js (uses chrono-node, dayjs)
  ├─ api.js (API client; exports only, no window.*)
  ├─ auth.js (auth flow)
  ├─ chat.js (chat functionality; uses context.js)
  ├─ profile.js (profile panel; uses panels/panel.js, cards/profile-content.js, context.js)
  ├─ app.js (bootstrap, wiring, API status, add-todo)
  ├─ todo-list.js (todo list rendering, drag-drop; uses cards/todo-card.js)
  ├─ context.js (single source of truth for AI context)
  ├─ panels/panel.js (panel helper; no external deps)
  ├─ cards/todo-card.js (todo card builder; uses dateutils.js)
  └─ cards/profile-content.js (profile panel body; uses html-utils.js)

index-entry.js
  ├─ config.js
  ├─ jwt.js
  ├─ api.js
  ├─ auth.js
  └─ app.js
```

**Note**: The build system bundles all dependencies, so HTML files only need to load the single bundle file (`dist/app.js` or `dist/index.js`).

## Module Structure

All JavaScript files are ES6 modules. The API module exports only; callers use ES module imports. The build system bundles these modules together.

### Key Modules

- **dateutils.js** - Date parsing and formatting using chrono-node and dayjs
  - `parseNaturalDate()` - Parse natural language dates
  - `formatDate()` - Format dates for display
  - `extractDateFromText()` - Extract dates from todo text
  - `isDateOnly()` - Check if a date is date-only (no time)

- **config.js** - API configuration loader
- **jwt.js** - JWT token management
- **api.js** - API client functions (exports only; no `window.*` assignments)
- **auth.js** - OIDC authentication flow
- **context.js** - Single source of truth for current AI context (`getContext`, `setContext`); used by chat and profile
- **panels/panel.js** - Panel helper (show/hide, setContent with loading/error/content); vanilla JS, no external dependency
- **cards/todo-card.js** - Builds a single todo card DOM node; used by todo-list.js
- **cards/profile-content.js** - Builds profile panel body content (user info, context textarea, tag stats, logout)
- **todo-list.js** - Todo list state, rendering, drag-drop, and todo actions; exports `loadTodos()`
- **chat.js** - AI chat interface; uses context.js
- **profile.js** - Profile panel (single load path, timeout, panel + profile-content builder); uses context.js
- **app.js** - Bootstrap, event wiring, API status indicator/dropdown, add-todo handler

## Quality Tools

The frontend includes comprehensive quality tooling:

- **ESLint** - Code linting with rules matching project style
- **c8** - Test coverage reporting (native Node.js coverage)
- **npm audit** - Security vulnerability scanning
- **Pre-commit hooks** - Automatic linting on commit
- **CI Integration** - All tools run in GitHub Actions

## Deployment

### Development Deployment

When deploying manually, ensure:
1. Build the frontend: `npm run build`
2. Include `dist/app.js` and `dist/index.js` in deployment
3. Deploy `config.json` with correct `api_base_url`
4. All other static assets (CSS, HTML, manifest.json) are included

### Production Release Packages

For production deployments, use the release packages created automatically when version tags are pushed:

1. **Docker Image**: Built via GoReleaser, available as `ghcr.io/benvon/smart-todo-api-web:latest`
2. **SPA/PWA Package**: Tarball created via GitHub Actions release workflow, available in GitHub Releases

The release package includes:
- Built `dist/` directory
- All static files (HTML, CSS, manifest.json)
- Example nginx configuration
- Deployment README with instructions
- Version information

See the main project README for more deployment details.
