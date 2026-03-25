# Memorix Dashboard

A web-based administrative dashboard for monitoring and managing Memorix memory service deployments.

## Quick Start

```bash
cd app
pnpm install
pnpm dev
```

The dashboard will be available at `http://localhost:5173`.

## Technology Stack

- **React 19** - UI framework
- **Vite 7** - Build tool
- **TypeScript 5** - Type safety
- **Tailwind CSS 4** - Styling
- **TanStack Router** - Routing
- **TanStack Query** - Data fetching
- **Recharts** - Data visualization
- **Radix UI + shadcn/ui** - UI components

## Project Structure

```
dashboard/
├── README.md
├── docs/
│   ├── dashboard-spec.md    # Product specification
│   └── data-contract.md     # API contract
└── app/
    ├── index.html
    ├── package.json
    ├── vite.config.ts
    ├── tsconfig.json
    ├── components.json      # shadcn/ui configuration
    └── src/
        ├── main.tsx         # Entry point
        ├── router.tsx       # Route definitions
        ├── index.css        # Global styles
        ├── pages/           # Page components
        ├── api/             # API client & queries
        ├── types/           # TypeScript types
        ├── lib/             # Utilities
        └── components/
            ├── ui/          # Base UI components
            └── dashboard/   # Dashboard components
```

## Development

### Commands

| Command | Description |
|---------|-------------|
| `pnpm dev` | Start development server |
| `pnpm build` | Build for production |
| `pnpm typecheck` | Run TypeScript type checking |
| `pnpm preview` | Preview production build |

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `VITE_API_PROXY_TARGET` | Backend API URL | `http://localhost:8080` |

### Authentication

The dashboard requires a dashboard token to access the API. The token is configured on the server via the `MNEMO_DASHBOARD_TOKEN` environment variable.

Enter the token on the login page to access the dashboard.

## Documentation

- [Product Specification](docs/dashboard-spec.md) - Features and design guidelines
- [API Contract](docs/data-contract.md) - Backend API endpoints and data types
