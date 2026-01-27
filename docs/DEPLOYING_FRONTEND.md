# Deploying the Frontend

## Automated deployment to Cloudflare Pages

On every version tag push (`v*.*.*` or `v*.*.*-rc*`), the release-web workflow:

1. Builds the frontend and creates a release tarball (without `config.json`)
2. Deploys the built site to Cloudflare Pages with production config injected from secrets
3. Creates a GitHub Release and attaches the tarball

The release tarball is intentionally built **without** `config.json`; config is deployment-specific and is supplied at deploy time (e.g. from GitHub Actions secrets for Cloudflare Pages, or manually for other hosts).

### Required GitHub Actions secrets

For the automated Cloudflare Pages deployment to succeed, configure these repository secrets:

| Secret | Purpose |
| ------ | ------- |
| `WEB_API_BASE_URL` | Production API base URL written into `config.json` (e.g. `https://api.example.com`) |
| `CLOUDFLARE_API_TOKEN` | API token with "Cloudflare Pages – Edit" (Cloudflare Dashboard → My Profile → API Tokens) |
| `CLOUDFLARE_ACCOUNT_ID` | Cloudflare account ID (Dashboard or API section) |
| `CLOUDFLARE_PAGES_PROJECT_NAME` | Name of the **Pages** project (not a Worker). See troubleshooting if you only have a Worker. |

The project name is the **exact** name of a **Cloudflare Pages** project shown in the dashboard (Workers & Pages → your **Pages** project). It is case-sensitive and must match what Cloudflare shows—not a GUID. If you see "Project not found" (API error 8000007), the project name or account ID is wrong for the account the token uses.

See [Frontend Deployment](../README.md#frontend-deployment) in the main README for manual build and deployment steps (including other hosts and `config.json` setup).

### Troubleshooting: "Project not found" (8000007)

1. **Confirm account and project**
   - In [Cloudflare Dashboard](https://dash.cloudflare.com) go to **Workers & Pages** and open your project.
   - The **account** (left rail or URL) is the one that must match `CLOUDFLARE_ACCOUNT_ID`.
   - The **project name** is in the project’s overview/settings and often in the URL (e.g. `…/pages/view/todo-dev` → use `todo-dev`). Use that exact string in `CLOUDFLARE_PAGES_PROJECT_NAME` (same casing, no extra spaces).

2. **List projects for your account**
   - To see the exact `name` values and confirm the account:
     ```bash
     curl -s -H "Authorization: Bearer YOUR_CF_API_TOKEN" \
       "https://api.cloudflare.com/client/v4/accounts/YOUR_ACCOUNT_ID/pages/projects" | jq '.result[] | {name}'
     ```
   - Use the `name` value from that output as `CLOUDFLARE_PAGES_PROJECT_NAME`. If the list is empty, the project lives in a different account—switch the account in the dashboard and use that account’s ID.

### Troubleshooting: "I have a Worker, not a Pages project"

This workflow deploys only to **Cloudflare Pages** projects. **Workers** and **Pages** are different resources in the same "Workers & Pages" area: the Pages API is `…/pages/projects/…` and does not include Workers.

- If the thing you think of as "todo-dev" is a **Worker** (e.g. created as a Worker, or it shows up as a Worker in the dashboard), it will **not** be listed by the Pages API and you will get "Project not found" when deploying. A token with "Workers and Pages" permission can still only deploy to **Pages** via this workflow.
- **Fix:** Use a **Pages** project name in `CLOUDFLARE_PAGES_PROJECT_NAME`. Either:
  1. **Create a Pages project** in the dashboard: Workers & Pages → Create application → **Pages** → "Direct Upload" (or Connect to Git). Give it a name (e.g. `todo-dev` or `todo-pages`). The workflow will deploy to that project. You can attach your custom domain (e.g. todo.benvon.dev) to that Pages project in its settings.
  2. **Or** reuse an existing Pages project: run the `curl` above for `…/pages/projects` and use one of the listed `name` values. Only **Pages** projects appear there; Workers do not.
- If your live site today is served by a **Worker**, you can point your domain at the new **Pages** project once it exists and start using the automated deploys there, or keep the Worker and run a separate deployment process for it (this repo’s workflow does not support deploying to Workers).

### Troubleshooting: 403 Forbidden

A 403 means the request is authenticated but not allowed. For Pages deployments, the usual cause is **token permissions**.

1. **Token must have Pages – Edit**
   - The token needs **Cloudflare Pages** with **Edit** (or both “Pages Read” and “Pages Write” in custom tokens).
   - “Workers and Pages” or “Edit Cloudflare Workers” alone is **not** enough for the Pages API; add **Cloudflare Pages – Edit** explicitly.
   - In the dashboard: [Account API tokens](https://dash.cloudflare.com/?to=/:account/api-tokens) → Create Token → **Create Custom Token** → under Permissions add **Cloudflare Pages** → **Edit**. Or clone the “Edit Cloudflare Workers” template and add **Cloudflare Pages** → **Edit** so both Workers and Pages are covered.

2. **Account scope**
   - When creating the token, set **Account Resources** to “Include” → “Specific account” → choose the account that owns the Pages project (the one in `CLOUDFLARE_ACCOUNT_ID`). If the token is limited to another account, Pages requests will return 403.

3. **Verify**
   - After updating the token, set the new value in the `CLOUDFLARE_API_TOKEN` secret and re-run the workflow.
