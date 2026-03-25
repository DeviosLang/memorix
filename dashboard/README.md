# Memorix Dashboard

A web-based administrative dashboard for monitoring and managing Memorix memory service deployments.

## Table of Contents

- [Quick Start](#quick-start)
- [Development Environment Setup](#development-environment-setup)
- [Production Deployment](#production-deployment)
- [Features](#features)
- [Configuration](#configuration)
- [Architecture](#architecture)
- [Troubleshooting](#troubleshooting)

## Quick Start

```bash
cd dashboard/app
pnpm install
pnpm dev
```

The dashboard will be available at `http://localhost:5173`.

### Prerequisites

| Requirement | Version | Install |
|-------------|---------|---------|
| Node.js | 18+ | `nvm install 18 && nvm use 18` |
| pnpm | 8+ | `npm install -g pnpm` |

## Development Environment Setup

### Step 1: Clone and Install Dependencies

```bash
git clone https://github.com/DeviosLang/memorix.git
cd memorix/dashboard/app
pnpm install
```

### Step 2: Configure Backend API

The dashboard proxies API requests to the memorix server. Configure the target:

```bash
# Option 1: Environment variable
export VITE_API_PROXY_TARGET="http://localhost:8080"
pnpm dev

# Option 2: .env file
echo 'VITE_API_PROXY_TARGET=http://localhost:8080' > .env
pnpm dev
```

### Step 3: Start memorix-server

The dashboard requires a running memorix server:

```bash
# From project root
cd ../../
make build
MNEMO_DSN="user:pass@tcp(host:4000)/memorix?parseTime=true" \
  MNEMO_DASHBOARD_TOKEN="your-secret-token" \
  make run
```

### Step 4: Configure Dashboard Token

Set the dashboard token on the server:

```bash
# Server-side configuration
export MNEMO_DASHBOARD_TOKEN="your-secret-token"
```

On the dashboard login page, enter this token to authenticate.

### Development Commands

| Command | Description |
|---------|-------------|
| `pnpm dev` | Start development server with hot reload |
| `pnpm build` | Build for production |
| `pnpm typecheck` | Run TypeScript type checking |
| `pnpm preview` | Preview production build locally |

## Production Deployment

### Option 1: Static File Deployment

Build the dashboard and serve static files:

```bash
# Build the dashboard
cd dashboard/app
pnpm build

# Output is in dist/ directory
# Serve with any static file server
```

#### Nginx Configuration

```nginx
server {
    listen 80;
    server_name dashboard.memorix.io;

    root /var/www/memorix-dashboard/dist;
    index index.html;

    # SPA fallback - all routes should serve index.html
    location / {
        try_files $uri $uri/ /index.html;
    }

    # Proxy API requests to memorix-server
    location /api/ {
        proxy_pass http://memorix-server:8080;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # Security headers
    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-XSS-Protection "1; mode=block" always;
}
```

#### Caddy Configuration

```caddyfile
dashboard.memorix.io {
    root * /var/www/memorix-dashboard/dist
    
    # SPA fallback
    try_files {path} /index.html
    file_server
    
    # API proxy
    handle /api/* {
        reverse_proxy memorix-server:8080
    }
}
```

### Option 2: Docker Deployment

```dockerfile
# dashboard/app/Dockerfile
FROM node:18-alpine AS builder
WORKDIR /app
RUN npm install -g pnpm
COPY package.json pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY . .
RUN pnpm build

FROM nginx:alpine
COPY --from=builder /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
```

Build and run:

```bash
cd dashboard/app
docker build -t memorix-dashboard .
docker run -p 80:80 memorix-dashboard
```

### Option 3: Kubernetes Deployment

```yaml
# dashboard-k8s.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: memorix-dashboard
spec:
  replicas: 2
  selector:
    matchLabels:
      app: memorix-dashboard
  template:
    metadata:
      labels:
        app: memorix-dashboard
    spec:
      containers:
      - name: dashboard
        image: memorix-dashboard:latest
        ports:
        - containerPort: 80
---
apiVersion: v1
kind: Service
metadata:
  name: memorix-dashboard
spec:
  selector:
    app: memorix-dashboard
  ports:
  - port: 80
    targetPort: 80
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: memorix-dashboard
spec:
  rules:
  - host: dashboard.memorix.io
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: memorix-dashboard
            port:
              number: 80
      - path: /api
        pathType: Prefix
        backend:
          service:
            name: memorix-server
            port:
              number: 8080
```

### API Proxy Configuration

The dashboard requires API requests to be proxied to the memorix server. Configure this based on your deployment:

| Deployment | Proxy Method |
|------------|--------------|
| Development | Vite dev server proxy (automatic) |
| Static + Nginx | Nginx `proxy_pass` directive |
| Static + Caddy | Caddy `reverse_proxy` directive |
| Kubernetes | Ingress path-based routing |

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `VITE_API_PROXY_TARGET` | Backend API URL (dev only) | `http://localhost:8080` |

Production builds should use relative API paths (`/api/...`) and rely on the web server for proxying.

## Features

### System Overview Dashboard

The main landing page provides an at-a-glance view of system health:

- **System Status**: Healthy / Degraded / Unhealthy indicator
- **Uptime**: Server start time and human-readable uptime
- **Request Statistics**:
  - Total requests count
  - Requests per second
  - Error rate percentage
  - Latency percentiles (P50, P95, P99)
- **Active Resources**:
  - Number of active tenants
  - Number of active agents

### Memory Statistics View

Detailed memory storage analytics:

- Total memory count across all tenants
- Distribution by state (active, archived, deleted)
- Distribution by type (fact, summary, experience)
- Storage metrics (total bytes, average content size)
- Top tenants by memory count

### Search Performance View

Search system analytics:

- Search counts by type (vector, keyword, hybrid, FTS)
- Search type distribution percentages
- Latency metrics (average, P50, P95, P99)
- Search success rate

### Garbage Collection View

GC operation monitoring:

- Last run information (time, ID, deleted count, duration)
- Historical totals (runs, deleted, recovered)
- Next scheduled run time
- Recent GC run history

### Tenant Management View

Tenant and agent statistics:

- Total, active, and suspended tenant counts
- Agent counts per tenant
- Top active tenants by request volume

### Conflict Resolution View

Conflict handling analytics:

- Total conflicts resolved
- Resolution type breakdown (LWW vs LLM merge)
- Merge success rate
- Recent conflict examples

## Configuration

### Server-Side Configuration

Configure the dashboard token on the memorix server:

```bash
# Required: Set a secure dashboard token
export MNEMO_DASHBOARD_TOKEN="your-secure-random-token"

# Optional: Configure dashboard access
export MNEMO_DASHBOARD_ENABLED="true"
```

### Dashboard Token Security

The dashboard token provides full administrative access. Follow these guidelines:

1. **Use a strong token**: Generate a cryptographically random token

   ```bash
   # Generate a secure token
   openssl rand -hex 32
   ```

2. **Store securely**: Use environment variables or secrets management

   ```bash
   # Kubernetes secret
   kubectl create secret generic memorix-dashboard-token \
     --from-literal=token=$(openssl rand -hex 32)
   ```

3. **Rotate regularly**: Change the token periodically

4. **Limit access**: Restrict who knows the token

### Client-Side Configuration

The dashboard stores configuration in browser localStorage:

| Key | Description |
|-----|-------------|
| `memorix_dashboard_token` | Authentication token |
| `memorix_dashboard_theme` | Theme preference (light/dark) |

### Theme Customization

The dashboard supports light and dark themes. Customize colors in `src/index.css`:

```css
:root {
  --background: 0 0% 100%;
  --foreground: 222.2 84% 4.9%;
  /* ... */
}

.dark {
  --background: 222.2 84% 4.9%;
  --foreground: 210 40% 98%;
  /* ... */
}
```

## Architecture

### Technology Stack

| Component | Technology | Version |
|-----------|-----------|---------|
| Framework | React | 19.x |
| Build Tool | Vite | 7.x |
| Language | TypeScript | 5.x |
| Styling | Tailwind CSS | 4.x |
| UI Components | Radix UI + shadcn/ui | Latest |
| Routing | TanStack Router | 1.x |
| Data Fetching | TanStack Query | 5.x |
| Charts | Recharts | 2.x |
| i18n | i18next | 25.x |

### Project Structure

```
dashboard/
├── README.md
├── docs/
│   ├── dashboard-spec.md    # Product specification
│   └── data-contract.md     # API contract
└── app/
    ├── index.html
    ├── package.json
    ├── vite.config.ts       # Vite configuration with API proxy
    ├── tsconfig.json
    ├── components.json      # shadcn/ui configuration
    └── src/
        ├── main.tsx         # App entry point
        ├── router.tsx       # Route definitions
        ├── index.css        # Global styles + Tailwind
        ├── pages/           # Page components
        │   ├── dashboard.tsx
        │   ├── storage.tsx
        │   ├── agents.tsx
        │   ├── spaces.tsx
        │   └── login.tsx
        ├── api/             # API client & queries
        │   ├── client.ts
        │   └── queries.ts
        ├── types/           # TypeScript types
        │   └── metrics.ts
        ├── lib/             # Utilities
        │   ├── utils.ts
        │   ├── i18n.ts
        │   ├── theme.ts
        │   └── session.ts
        ├── i18n/            # Internationalization
        │   ├── index.ts
        │   └── locales/
        │       ├── en.json
        │       └── zh-CN.json
        └── components/
            ├── ui/          # Base UI components (shadcn)
            │   ├── button.tsx
            │   └── card.tsx
            ├── sidebar.tsx
            ├── theme-toggle.tsx
            └── locale-toggle.tsx
```

### API Endpoints

The dashboard communicates with memorix-server via these endpoints:

| Endpoint | Description |
|----------|-------------|
| `GET /api/dashboard/overview` | System overview |
| `GET /api/dashboard/memory-stats` | Memory statistics |
| `GET /api/dashboard/search-stats` | Search statistics |
| `GET /api/dashboard/gc-stats` | GC statistics |
| `GET /api/dashboard/space-stats` | Tenant/agent statistics |
| `GET /api/dashboard/conflict-stats` | Conflict statistics |

All endpoints require the `X-Dashboard-Token` header for authentication.

### Data Flow

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│  React Pages    │────>│  TanStack Query │────>│  API Client     │
│  (dashboard.tsx)│     │  (queries.ts)   │     │  (client.ts)    │
└─────────────────┘     └─────────────────┘     └────────┬────────┘
                                                         │
                                                         ▼
                        ┌─────────────────────────────────────────┐
                        │           memorix-server                │
                        │  /api/dashboard/* endpoints             │
                        │  (requires X-Dashboard-Token header)    │
                        └─────────────────────────────────────────┘
```

### Authentication Flow

1. User enters dashboard token on login page
2. Token is stored in localStorage
3. API client adds `X-Dashboard-Token` header to all requests
4. Server validates token and returns data
5. Session expires when token is cleared or page is closed

## Troubleshooting

### Common Issues

#### Authentication Failed

**Symptoms**: Login page shows "Invalid token" error

**Solutions**:
1. Verify the token matches server configuration
2. Check that `MNEMO_DASHBOARD_TOKEN` is set on the server
3. Clear browser localStorage and try again

```bash
# Verify server token
echo $MNEMO_DASHBOARD_TOKEN

# Restart server with token
export MNEMO_DASHBOARD_TOKEN="your-token"
make run
```

#### API Proxy Not Working

**Symptoms**: Network errors, 404s, or CORS issues

**Solutions**:

For development:
```bash
# Check VITE_API_PROXY_TARGET
echo $VITE_API_PROXY_TARGET

# Start with correct proxy
VITE_API_PROXY_TARGET=http://localhost:8080 pnpm dev
```

For production:
```nginx
# Verify Nginx proxy configuration
location /api/ {
    proxy_pass http://memorix-server:8080;
}
```

#### Blank Page After Build

**Symptoms**: Production build shows blank page

**Solutions**:
1. Check browser console for errors
2. Verify `base` path in `vite.config.ts` if deploying to subdirectory
3. Ensure static files are served with correct MIME types

```typescript
// vite.config.ts for subdirectory deployment
export default defineConfig({
  base: '/dashboard/',
  // ...
});
```

#### Data Not Refreshing

**Symptoms**: Dashboard shows stale data

**Solutions**:
1. Check TanStack Query stale time configuration
2. Verify server is returning fresh data
3. Use manual refresh button

```typescript
// Adjust refresh interval in queries.ts
useQuery({
  queryKey: ['overview'],
  queryFn: fetchOverview,
  staleTime: 30 * 1000, // 30 seconds
  refetchInterval: 60 * 1000, // 1 minute
});
```

### Debug Mode

Enable debug logging in the browser console:

```javascript
// In browser console
localStorage.setItem('memorix_debug', 'true');
location.reload();
```

### Health Check

Verify dashboard connectivity:

```bash
# Check API endpoint directly
curl -H "X-Dashboard-Token: your-token" \
  http://localhost:8080/api/dashboard/overview

# Expected response
{"status":"healthy",...}
```

### Getting Help

1. Check [dashboard-spec.md](docs/dashboard-spec.md) for feature details
2. Check [data-contract.md](docs/data-contract.md) for API specifications
3. Review [../docs/DESIGN.md](../docs/DESIGN.md) for architecture
4. Open an issue at https://github.com/DeviosLang/memorix/issues

## Internationalization

The dashboard supports multiple languages:

| Language | Code | Status |
|----------|------|--------|
| English | `en` | Complete |
| Chinese (Simplified) | `zh-CN` | Complete |

Add a new language:

1. Create `src/i18n/locales/YOUR_LANG.json`
2. Add to `src/i18n/index.ts`:

```typescript
import yourLang from './locales/YOUR_LANG.json';

i18n.use(initReactI18next).init({
  resources: {
    'YOUR_LANG': { translation: yourLang },
    // ...
  },
});
```

## Contributing

See the main [CONTRIBUTING.md](../CONTRIBUTING.md) for development guidelines.

### Development Tips

1. **Type checking**: Run `pnpm typecheck` before committing
2. **Component library**: Use shadcn/ui components from `src/components/ui/`
3. **Styling**: Use Tailwind utility classes, avoid custom CSS
4. **State management**: Use TanStack Query for server state
