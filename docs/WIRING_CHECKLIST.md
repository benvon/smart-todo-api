# Application Wiring Checklist

## Frontend (Web) ✅

### HTML Files - Script Dependencies

**index.html** ✅
- ✅ config.js (line 25) - Sets API base URL
- ✅ jwt.js (line 26) - Token utilities (storeToken, getToken, removeToken, isTokenExpired)
- ✅ api.js (line 27) - API client functions (getOIDCLoginConfig, apiRequest, etc.)
- ✅ auth.js (line 28) - Auth flow (initiateLogin, handleCallback, logout, isAuthenticated)
- ✅ app.js (line 29) - App initialization and event handlers

**app.html** ✅
- ✅ config.js (line 40) - Sets API base URL
- ✅ jwt.js (line 41) - Token utilities
- ✅ api.js (line 42) - API client functions
- ✅ auth.js (line 43) - Auth flow
- ✅ app.js (line 44) - App logic

### JavaScript Dependencies

| Module | Dependencies | Used By |
|--------|-------------|---------|
| config.js | None | Sets `window.API_BASE_URL` |
| jwt.js | None | Defines: `storeToken`, `getToken`, `removeToken`, `isTokenExpired`, `parseToken`, `getTokenExpiration` |
| api.js | jwt.js, config.js | Defines: `apiRequest`, `getOIDCLoginConfig`, `getCurrentUser`, `getTodos`, `createTodo`, `updateTodo`, `deleteTodo`, `completeTodo` |
| auth.js | api.js, jwt.js | Defines: `initiateLogin`, `handleCallback`, `exchangeCodeForToken`, `logout`, `isAuthenticated`, `generateState`, `showError` |
| app.js | auth.js, api.js | Defines: `loadTodos`, `handleAddTodo`, `handleCompleteTodo`, `handleDeleteTodo`, `renderTodos`, `renderTodoList`, `escapeHtml`, `showError` |

### Frontend API Endpoints Used

- ✅ `GET /api/v1/auth/oidc/login` - Get OIDC config (used by `auth.js`)
- ✅ `GET /api/v1/auth/me` - Get current user (defined in `api.js` but not actively used yet)
- ✅ `GET /api/v1/todos` - Get todos (used by `app.js`)
- ✅ `POST /api/v1/todos` - Create todo (used by `app.js`)
- ✅ `PATCH /api/v1/todos/:id` - Update todo (defined in `api.js` but not used)
- ✅ `DELETE /api/v1/todos/:id` - Delete todo (used by `app.js`)
- ✅ `POST /api/v1/todos/:id/complete` - Complete todo (used by `app.js`)

### Function Cross-References

**Critical Dependencies (must be loaded before use):**
- `storeToken` (jwt.js) → used in auth.js:75 ✅
- `getToken` (jwt.js) → used in api.js:7, auth.js:151 ✅
- `removeToken` (jwt.js) → used in auth.js:143, auth.js:157 ✅
- `isTokenExpired` (jwt.js) → used in auth.js:156 ✅
- `getOIDCLoginConfig` (api.js) → used in auth.js:8, auth.js:66 ✅
- `initiateLogin` (auth.js) → used in app.js:19 ✅
- `handleCallback` (auth.js) → used in app.js:12 ✅
- `isAuthenticated` (auth.js) → used in app.js:25 ✅
- `logout` (auth.js) → used in app.js:33 ✅

## Backend (Server) ✅

### Route Registration

**Public Routes:**
- ✅ `GET /healthz` → `healthChecker.HealthCheck`
- ✅ `GET /health` → `healthCheck` (legacy)
- ✅ `GET /version` → `versionInfo`
- ✅ `GET /api/v1/openapi.yaml` → `openAPIHandler.ServeYAML`
- ✅ `GET /api/v1/openapi.json` → `openAPIHandler.ServeJSON`

**Public Auth Routes:**
- ✅ `GET /api/v1/auth/oidc/login` → `authHandler.GetOIDCLogin` (no auth middleware)

**Protected Auth Routes:**
- ✅ `GET /api/v1/auth/me` → `authHandler.GetMe` (requires auth middleware)

**Protected Todo Routes:**
- ✅ Registered via `todoHandler.RegisterRoutes(todosRouter)` with auth middleware

### Middleware Stack

Applied in order (cmd/server/main.go:52-55):
1. ✅ `middleware.Logging` - Request logging
2. ✅ `middleware.ErrorHandler` - Error handling
3. ✅ `middleware.CORSFromEnv` - CORS headers

Applied to protected routes:
- ✅ `middleware.Auth` - JWT token validation

### Handler Dependencies

**AuthHandler:**
- ✅ Requires: `oidcProvider` (initialized in main.go:41)
- ✅ Routes: `/oidc/login` (public), `/me` (protected)

**TodoHandler:**
- ✅ Requires: `todoRepo` (initialized in main.go:37)
- ✅ Routes: Registered via `RegisterRoutes` method

**HealthChecker:**
- ✅ Requires: `db` (initialized in main.go:30)
- ✅ Route: `/healthz`

## Database Schema ✅

**Migrations:**
- ✅ `000001_initial_schema` - Initial schema (users, todos, oidc_config tables)
- ✅ `000002_make_client_secret_nullable` - Makes client_secret nullable
- ✅ `000003_add_domain_to_oidc_config` - Adds domain column for OAuth2 domains

**Expected Tables:**
- ✅ `users` - User accounts
- ✅ `todos` - Todo items
- ✅ `oidc_config` - OIDC provider configuration (with `domain` column)

## Configuration ✅

**Environment Variables (docker-compose.yml):**
- ✅ `DATABASE_URL` - Database connection string
- ✅ `SERVER_PORT` - Server port (default: 8080)
- ✅ `BASE_URL` - Backend base URL (for internal use)
- ✅ `FRONTEND_URL` - Frontend URL (for CORS)
- ✅ `API_BASE_URL` - Frontend API base URL (in web service)

**Frontend Config (config.json):**
- ✅ Generated from `API_BASE_URL` environment variable at runtime
- ✅ Served at `/config.json` via nginx custom location

## Potential Issues Fixed

1. ✅ **Missing `api.js` in index.html** - FIXED (was missing, now added)
2. ✅ **Missing `jwt.js` in index.html** - FIXED (was missing, now added)
3. ✅ **Duplicate `showError` function** - Both auth.js and app.js define it (works but redundant)
4. ✅ **Token endpoint not included in backend response** - FIXED (added `token_endpoint` to LoginConfig)
5. ✅ **Double prefix in todo routes** - FIXED (RegisterRoutes was creating `/api/v1/todos` prefix on already-prefixed router)

## Verification Steps

To verify everything is wired correctly:

1. **Check HTML files have all scripts in correct order:**
   ```bash
   grep -n "script src" web/*.html
   ```
   Should show: config.js, jwt.js, api.js, auth.js, app.js in that order for both files.

2. **Verify all functions are defined before use:**
   - `getToken`, `storeToken`, etc. from jwt.js must load before auth.js
   - `getOIDCLoginConfig` from api.js must load before auth.js
   - All auth functions must load before app.js

3. **Test API endpoints:**
   - `GET /api/v1/auth/oidc/login` should return OIDC config with `token_endpoint`
   - `GET /api/v1/auth/me` should require Authorization header
   - All `/api/v1/todos/*` routes should require Authorization header

4. **Verify database schema:**
   - Run migrations to ensure `oidc_config.domain` column exists
   - Verify OIDC config can be stored with domain field
