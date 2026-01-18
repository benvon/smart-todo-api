# Frontend JavaScript Dependencies

## File Dependency Graph

```
config.js (no deps)
  └─ Sets window.API_BASE_URL

jwt.js (no deps)
  └─ Defines: storeToken, getToken, removeToken, isTokenExpired, parseToken, getTokenExpiration

api.js (depends on: jwt.js, config.js)
  ├─ Uses: getToken, window.API_BASE_URL
  └─ Defines: apiRequest, getOIDCLoginConfig, getCurrentUser, getTodos, createTodo, updateTodo, deleteTodo, completeTodo

auth.js (depends on: api.js, jwt.js)
  ├─ Uses: getOIDCLoginConfig, storeToken, getToken, isTokenExpired, removeToken
  └─ Defines: initiateLogin, handleCallback, exchangeCodeForToken, logout, isAuthenticated, generateState, showError

app.js (depends on: auth.js, api.js)
  ├─ Uses: handleCallback, initiateLogin, isAuthenticated, logout, getTodos, createTodo, completeTodo, deleteTodo
  └─ Defines: loadTodos, handleAddTodo, handleCompleteTodo, handleDeleteTodo, renderTodos, renderTodoList, escapeHtml, showError
```

## Required Script Load Order

**index.html and app.html must load scripts in this order:**
1. `config.js` - Sets API base URL (no dependencies)
2. `jwt.js` - Token utilities (no dependencies)  
3. `api.js` - API client (needs jwt.js, config.js)
4. `auth.js` - Auth flow (needs api.js, jwt.js)
5. `app.js` - App logic (needs auth.js, api.js)

## Function Usage Matrix

| Function | Defined In | Used By |
|----------|-----------|---------|
| `storeToken` | jwt.js | auth.js:75 |
| `getToken` | jwt.js | api.js:7, auth.js:151 |
| `removeToken` | jwt.js | auth.js:143, auth.js:157 |
| `isTokenExpired` | jwt.js | auth.js:156 |
| `getOIDCLoginConfig` | api.js | auth.js:8, auth.js:66 |
| `apiRequest` | api.js | api.js:66, api.js:73, api.js:90, api.js:100, api.js:110, api.js:119 |
| `initiateLogin` | auth.js | app.js:19 |
| `handleCallback` | auth.js | app.js:12 |
| `logout` | auth.js | app.js:33 |
| `isAuthenticated` | auth.js | app.js:25 |
| `getTodos` | api.js | app.js:61 |
| `createTodo` | api.js | app.js:82 |
| `completeTodo` | api.js | app.js:97 |
| `deleteTodo` | api.js | app.js:122 |
| `showError` | auth.js, app.js | auth.js:35, auth.js:49, auth.js:60, auth.js:79, auth.js:83, app.js:66, app.js:88, app.js:109, app.js:129 |

## Potential Issues

1. **Duplicate `showError` function** - Defined in both `auth.js` and `app.js`. This should work but is redundant.
2. **Global scope functions** - All functions are global. Consider namespacing for better organization.
