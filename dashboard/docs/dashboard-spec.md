# Memorix Dashboard Product Specification

**Version**: 0.1.0  
**Status**: Draft  
**Last Updated**: 2026-03-26

## Overview

The Memorix Dashboard is a web-based administrative interface for monitoring and managing a Memorix memory service deployment. Unlike the mem9 dashboard which focuses on end-user memory management, the Memorix Dashboard is designed as a **system operations panel** targeting DevOps engineers and system administrators.

## Target Users

- **DevOps Engineers**: Monitor system health, performance metrics, and resource usage
- **System Administrators**: Manage tenants, view audit logs, and configure system settings
- **Support Teams**: Debug issues, view tenant activity, and analyze search performance

## Core Features

### 1. Authentication & Access Control

- Token-based authentication using dashboard tokens
- Single-user admin access (no multi-user RBAC in MVP)
- Secure token storage in localStorage
- Automatic session validation

### 2. System Overview Dashboard

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

### 3. Memory Statistics View

Detailed memory storage analytics:

- Total memory count across all tenants
- Distribution by state (active, archived, deleted)
- Distribution by type (fact, summary, experience)
- Storage metrics (total bytes, average content size)
- Top tenants by memory count

### 4. Search Performance View

Search system analytics:

- Search counts by type (vector, keyword, hybrid, FTS)
- Search type distribution percentages
- Latency metrics (average, P50, P95, P99)
- Search success rate

### 5. Garbage Collection View

GC operation monitoring:

- Last run information (time, ID, deleted count, duration)
- Historical totals (runs, deleted, recovered)
- Next scheduled run time
- Recent GC run history

### 6. Tenant Management View

Tenant and agent statistics:

- Total, active, and suspended tenant counts
- Agent counts per tenant
- Top active tenants by request volume

### 7. Conflict Resolution View

Conflict handling analytics:

- Total conflicts resolved
- Resolution type breakdown (LWW vs LLM merge)
- Merge success rate
- Recent conflict examples

## Technical Architecture

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

### Project Structure

```
dashboard/
├── README.md
├── docs/
│   ├── dashboard-spec.md          # This document
│   └── data-contract.md           # API contract
├── app/
│   ├── index.html
│   ├── package.json
│   ├── vite.config.ts
│   ├── tsconfig.json
│   ├── components.json            # shadcn/ui config
│   └── src/
│       ├── main.tsx               # App entry point
│       ├── router.tsx             # Route definitions
│       ├── index.css              # Global styles
│       ├── pages/                 # Page components
│       ├── api/                   # API client & queries
│       ├── types/                 # TypeScript types
│       ├── lib/                   # Utilities
│       └── components/
│           ├── ui/                # Base UI components
│           └── dashboard/         # Dashboard-specific components
```

### API Integration

The dashboard communicates with the Memorix server via the Dashboard API endpoints:

- `GET /api/dashboard/overview` - System overview
- `GET /api/dashboard/memory-stats` - Memory statistics
- `GET /api/dashboard/search-stats` - Search statistics
- `GET /api/dashboard/gc-stats` - GC statistics
- `GET /api/dashboard/space-stats` - Tenant/agent statistics
- `GET /api/dashboard/conflict-stats` - Conflict statistics

All endpoints require the `X-Dashboard-Token` header for authentication.

### Data Refresh Strategy

- Overview data: Refresh every 30 seconds
- Detailed stats: Refresh every 60 seconds
- Manual refresh option on each page
- Real-time updates not included in MVP

## Design Guidelines

### Visual Design

- Clean, minimal interface with focus on data clarity
- Dark mode support with CSS variables
- Responsive layout (desktop-first for operations use)
- Consistent spacing and typography using Tailwind

### Navigation

- Left sidebar navigation with section links
- Current page highlighted
- Logout option at bottom of sidebar

### Error Handling

- Graceful degradation when API is unavailable
- Clear error messages for authentication failures
- Loading states for async operations

## Future Enhancements (Post-MVP)

- Real-time metrics via WebSocket
- Historical data charts with time range selection
- Tenant detail pages with drill-down analytics
- Configuration management UI
- Audit log viewer
- Multi-user access with RBAC
- Custom dashboard layouts
- Export reports functionality

## Success Metrics

- Dashboard loads and displays data within 2 seconds
- System status updates within 30 seconds of state change
- Zero authentication bypass vulnerabilities
- Clear error messaging for all failure modes
