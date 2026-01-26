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
| `CLOUDFLARE_PAGES_PROJECT_NAME` | Name of the Cloudflare Pages project for this frontend |

See [Frontend Deployment](../README.md#frontend-deployment) in the main README for manual build and deployment steps (including other hosts and `config.json` setup).
