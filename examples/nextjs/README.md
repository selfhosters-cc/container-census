# Container Census Next.js Integration

This directory contains ready-to-use components and utilities for integrating Container Census telemetry data into your Next.js application.

## Table of Contents

- [Overview](#overview)
- [Prerequisites](#prerequisites)
- [Installation](#installation)
- [Configuration](#configuration)
- [Usage](#usage)
  - [Quick Start](#quick-start)
  - [API Client](#api-client)
  - [Server Components](#server-components)
  - [Client Components](#client-components)
- [Available Components](#available-components)
- [API Reference](#api-reference)
- [Security Best Practices](#security-best-practices)
- [Deployment](#deployment)
- [Troubleshooting](#troubleshooting)

## Overview

This integration provides:

- **Type-safe API client** for fetching telemetry data
- **Server Components** for SSR performance and SEO
- **Client Components** with interactive Chart.js visualizations
- **Pre-built charts** matching the telemetry collector dashboard
- **API key authentication** for secure data access

## Prerequisites

- Node.js 18+ (for Next.js App Router)
- Next.js 13.4+ (with App Router)
- Access to a Container Census telemetry collector instance
- API key for stats endpoints (set via `STATS_API_KEY` env var on collector)

## Installation

### 1. Copy Files to Your Project

Copy the contents of this directory to your Next.js project:

```bash
cp -r examples/nextjs/lib/* your-nextjs-app/lib/
cp -r examples/nextjs/components/* your-nextjs-app/components/
cp -r examples/nextjs/app/* your-nextjs-app/app/
```

### 2. Install Dependencies

```bash
npm install chart.js
# or
yarn add chart.js
# or
pnpm add chart.js
```

### 3. Configure TypeScript (if needed)

Ensure your `tsconfig.json` includes:

```json
{
  "compilerOptions": {
    "paths": {
      "@/*": ["./*"]
    }
  }
}
```

Or update import paths in the copied files to match your project structure.

## Configuration

### Environment Variables

Create a `.env.local` file in your Next.js project root:

```bash
# Required: Telemetry collector API base URL
TELEMETRY_API_URL=https://telemetry.example.com

# Required: API key for authentication
TELEMETRY_API_KEY=your-api-key-here
```

**Important**: These environment variables are **server-side only** and will not be exposed to the browser. This keeps your API key secure.

### Telemetry Collector Setup

On your telemetry collector instance, set the following environment variables:

```bash
# Generate a secure random API key
STATS_API_KEY=your-secure-api-key-here

# Optional: Enable Basic Auth for the dashboard UI
COLLECTOR_AUTH_ENABLED=true
COLLECTOR_AUTH_USERNAME=admin
COLLECTOR_AUTH_PASSWORD=secure-password
```

To generate a secure API key:

```bash
# Linux/macOS
openssl rand -hex 32

# Or use Node.js
node -e "console.log(require('crypto').randomBytes(32).toString('hex'))"
```

## Usage

### Quick Start

The easiest way to get started is to use the pre-built dashboard page:

1. Copy `app/telemetry/page.tsx` to your Next.js app
2. Visit `http://localhost:3000/telemetry` to see the dashboard

### API Client

The API client provides type-safe methods for fetching telemetry data:

```typescript
import { createTelemetryAPI } from '@/lib/telemetry-api';

// In a Server Component
export default async function MyPage() {
  const api = createTelemetryAPI();
  const summary = await api.getSummary();

  return (
    <div>
      <h1>Total Installations: {summary.installations}</h1>
      <p>Containers: {summary.total_containers}</p>
    </div>
  );
}
```

### Server Components

Server Components fetch data at build time or on request, providing better SEO and initial load performance:

```typescript
// app/stats/page.tsx
import { TelemetryStats } from '@/components/TelemetryStats';

export default function StatsPage() {
  return (
    <div>
      <h1>Telemetry Statistics</h1>
      <TelemetryStats />
    </div>
  );
}
```

### Client Components

Client Components enable interactive charts with tooltips and animations:

```typescript
// app/charts/page.tsx
import { createTelemetryAPI } from '@/lib/telemetry-api';
import { TopImagesChart } from '@/components/TopImagesChart';

export default async function ChartsPage() {
  const api = createTelemetryAPI();
  const images = await api.getTopImages({ limit: 10 });

  return (
    <div>
      <h1>Top Container Images</h1>
      <TopImagesChart images={images} />
    </div>
  );
}
```

## Available Components

### Server Components

#### `TelemetryStats`

Displays summary statistics in a card grid.

```typescript
import { TelemetryStats } from '@/components/TelemetryStats';

<TelemetryStats />
```

### Client Components (Interactive Charts)

All chart components accept data fetched from the API and render interactive Chart.js visualizations.

#### `TopImagesChart`

Horizontal bar chart of most popular container images.

```typescript
import { TopImagesChart } from '@/components/TopImagesChart';

<TopImagesChart images={data} title="Most Popular Images" />
```

**Props:**
- `images: ImageCount[]` - Array of image usage data
- `title?: string` - Chart title (default: "Top Container Images")

#### `GrowthChart`

Line chart showing installation growth and average container count over time.

```typescript
import { GrowthChart } from '@/components/GrowthChart';

<GrowthChart data={growth} title="Growth Trends" />
```

**Props:**
- `data: Growth[]` - Array of growth data points
- `title?: string` - Chart title (default: "Growth Over Time")

#### `RegistryChart`

Doughnut chart showing container registry distribution.

```typescript
import { RegistryChart } from '@/components/RegistryChart';

<RegistryChart data={registries} />
```

**Props:**
- `data: RegistryCount[]` - Array of registry usage data
- `title?: string` - Chart title (default: "Registry Distribution")

#### `VersionChart`

Bar chart showing Container Census version adoption.

```typescript
import { VersionChart } from '@/components/VersionChart';

<VersionChart data={versions} />
```

**Props:**
- `data: VersionCount[]` - Array of version usage data
- `title?: string` - Chart title (default: "Version Adoption")

#### `GeographyChart`

Bar chart showing geographic distribution based on timezone data.

```typescript
import { GeographyChart } from '@/components/GeographyChart';

<GeographyChart data={geography} />
```

**Props:**
- `data: GeographyData[]` - Array of geographic data
- `title?: string` - Chart title (default: "Geographic Distribution")

#### `ContainerImagesTable`

Interactive, sortable, filterable table displaying detailed container image data with pagination.

```typescript
import { ContainerImagesTable } from '@/components/ContainerImagesTable';

<ContainerImagesTable images={imageDetails} title="All Container Images" />
```

**Props:**
- `images: ImageDetail[]` - Array of detailed image data (includes name, count, registry, installation count)
- `title?: string` - Table title (default: "Container Images")

**Features:**
- Client-side search/filtering by image name
- Sortable columns (name, container count)
- Pagination (50 items per page)
- Color-coded registry badges (Docker Hub, GHCR, Quay, GCR, MCR)
- Percentage of total containers calculation
- Installation count per image
- Responsive design with Tailwind CSS

## API Reference

### TelemetryAPI Class

```typescript
import { TelemetryAPI } from '@/lib/telemetry-api';

const api = new TelemetryAPI({
  baseURL: 'https://telemetry.example.com',
  apiKey: 'your-api-key'
});
```

#### Methods

| Method | Parameters | Returns | Description |
|--------|------------|---------|-------------|
| `getSummary()` | - | `Promise<Summary>` | Get overview statistics |
| `getTopImages()` | `{ limit?, days? }` | `Promise<ImageCount[]>` | Get most popular images |
| `getGrowth()` | `{ days? }` | `Promise<Growth[]>` | Get growth metrics |
| `getRegistries()` | `{ days? }` | `Promise<RegistryCount[]>` | Get registry distribution |
| `getVersions()` | - | `Promise<VersionCount[]>` | Get version distribution |
| `getGeography()` | - | `Promise<GeographyData[]>` | Get geographic distribution |
| `getActivityHeatmap()` | `{ days? }` | `Promise<HeatmapData[]>` | Get activity heatmap data |
| `getScanIntervals()` | - | `Promise<IntervalCount[]>` | Get scan interval distribution |
| `getRecentEvents()` | `{ limit?, since? }` | `Promise<SubmissionEvent[]>` | Get recent submissions |
| `getInstallations()` | `{ days? }` | `Promise<{...}>` | Get installation count |
| `getImageDetails()` | `{ limit?, offset?, days?, search?, sort_by?, sort_order? }` | `Promise<ImageDetailsResponse>` | Get detailed image data with pagination and search |

### Data Types

All TypeScript types are exported from `@/lib/telemetry-api`:

```typescript
import {
  Summary,
  ImageCount,
  Growth,
  RegistryCount,
  VersionCount,
  GeographyData,
  HeatmapData,
  IntervalCount,
  SubmissionEvent,
  ImageDetail,
  ImageDetailsResponse
} from '@/lib/telemetry-api';
```

**New Types for Image Details:**

```typescript
interface ImageDetail {
  image: string;              // Container image name (normalized)
  count: number;              // Number of containers using this image
  registry: string;           // Registry source (Docker Hub, GHCR, etc.)
  installation_count: number; // Number of installations using this image
}

interface ImageDetailsResponse {
  images: ImageDetail[];
  pagination: {
    total: number;   // Total number of images
    limit: number;   // Page size
    offset: number;  // Current offset
  };
}
```

## Security Best Practices

### 1. Keep API Keys Server-Side

✅ **DO**: Use environment variables accessed only in Server Components

```typescript
// Server Component - ✅ SAFE
const api = createTelemetryAPI(); // Uses process.env
```

❌ **DON'T**: Expose API keys in Client Components or browser code

```typescript
// Client Component - ❌ UNSAFE
'use client';
const apiKey = process.env.TELEMETRY_API_KEY; // This won't work anyway
```

### 2. Use Next.js Environment Variable Conventions

- Prefix public variables with `NEXT_PUBLIC_` (but **don't** do this for API keys)
- Server-only variables (like `TELEMETRY_API_KEY`) are automatically protected

### 3. Implement Rate Limiting

For public-facing sites, implement your own rate limiting:

```typescript
// middleware.ts
import { NextResponse } from 'next/server';
import { Ratelimit } from '@upstash/ratelimit';

const ratelimit = new Ratelimit({
  redis: Redis.fromEnv(),
  limiter: Ratelimit.slidingWindow(10, '10 s'),
});

export async function middleware(request: Request) {
  if (request.url.includes('/telemetry')) {
    const ip = request.headers.get('x-forwarded-for') ?? 'anonymous';
    const { success } = await ratelimit.limit(ip);

    if (!success) {
      return new NextResponse('Rate limit exceeded', { status: 429 });
    }
  }

  return NextResponse.next();
}
```

### 4. Use HTTPS in Production

Always use HTTPS for your telemetry collector API in production:

```bash
# ✅ GOOD
TELEMETRY_API_URL=https://telemetry.example.com

# ❌ BAD (development only)
TELEMETRY_API_URL=http://telemetry.example.com
```

## Deployment

### Vercel

1. Add environment variables in Project Settings → Environment Variables:
   - `TELEMETRY_API_URL`
   - `TELEMETRY_API_KEY`

2. Deploy:
   ```bash
   vercel
   ```

### Docker

```dockerfile
FROM node:18-alpine

WORKDIR /app
COPY package*.json ./
RUN npm ci

COPY . .
RUN npm run build

ENV TELEMETRY_API_URL=https://telemetry.example.com
ENV TELEMETRY_API_KEY=your-api-key

EXPOSE 3000
CMD ["npm", "start"]
```

### Static Export (ISR)

For static sites with Incremental Static Regeneration:

```typescript
// app/telemetry/page.tsx
export const revalidate = 300; // Revalidate every 5 minutes

export default async function Page() {
  const api = createTelemetryAPI();
  const data = await api.getSummary();

  return <div>{/* ... */}</div>;
}
```

## Troubleshooting

### "TELEMETRY_API_URL is not set" Error

**Problem**: Environment variable not loaded

**Solutions**:
- Ensure `.env.local` exists in project root
- Restart Next.js dev server (`npm run dev`)
- Check variable names (no typos)

### "Invalid or missing API key" (401 Error)

**Problem**: API key authentication failed

**Solutions**:
- Verify `TELEMETRY_API_KEY` matches `STATS_API_KEY` on collector
- Check for whitespace in environment variables
- Restart both Next.js and telemetry collector services

### CORS Errors in Browser

**Problem**: Cross-origin requests blocked

**Solution**: CORS is handled by the API middleware, but verify:
- Using correct API endpoints (should include `/api/stats/`)
- API requests from Server Components (not Client Components making direct fetch calls)

If you need Client Components to fetch directly:

```typescript
// Create an API route proxy
// app/api/telemetry/[...path]/route.ts
import { createTelemetryAPI } from '@/lib/telemetry-api';

export async function GET(request: Request) {
  const api = createTelemetryAPI();
  // Forward request to telemetry API
  // This keeps API key server-side
}
```

### Charts Not Rendering

**Problem**: Chart.js not loading or canvas errors

**Solutions**:
- Ensure `chart.js` is installed: `npm list chart.js`
- Check browser console for JavaScript errors
- Verify component is marked with `'use client'` directive
- Confirm data is not empty

### Stale Data Showing

**Problem**: Data not updating

**Solutions**:
- Check `revalidate` setting in page component
- Clear Next.js cache: `rm -rf .next`
- Verify telemetry collector is receiving new data

## Examples

### Custom Styling

Customize chart colors to match your brand:

```typescript
// components/BrandedTopImagesChart.tsx
'use client';

import { TopImagesChart } from '@/components/TopImagesChart';
import type { ImageCount } from '@/lib/telemetry-api';

// Override colorPalette with your brand colors
const brandColors = ['#FF0000', '#00FF00', '#0000FF'];

export function BrandedTopImagesChart({ images }: { images: ImageCount[] }) {
  // Modify the component to use brandColors
  return <TopImagesChart images={images} />;
}
```

### Filtering Data

Show only specific data:

```typescript
export default async function FilteredPage() {
  const api = createTelemetryAPI();
  const allImages = await api.getTopImages({ limit: 100 });

  // Filter to only show official images
  const officialImages = allImages.filter(img =>
    !img.image.includes('/')
  );

  return <TopImagesChart images={officialImages} />;
}
```

### Multiple Dashboards

Create different views for different audiences:

```typescript
// app/public-stats/page.tsx - Public view
export default async function PublicStats() {
  const api = createTelemetryAPI();
  const summary = await api.getSummary();

  return (
    <div>
      <h1>{summary.installations} Installations</h1>
      <p>Join the community!</p>
    </div>
  );
}

// app/admin/analytics/page.tsx - Admin view
import { headers } from 'next/headers';

export default async function AdminAnalytics() {
  // Check authentication
  const headersList = headers();
  const session = await getSession(headersList);

  if (!session?.isAdmin) {
    return <div>Unauthorized</div>;
  }

  // Full analytics dashboard
  const api = createTelemetryAPI();
  const [summary, growth, images] = await Promise.all([
    api.getSummary(),
    api.getGrowth({ days: 365 }),
    api.getTopImages({ limit: 50 })
  ]);

  return (
    <div>
      {/* Comprehensive dashboard */}
    </div>
  );
}
```

## Support

For issues or questions:

- **Container Census Issues**: https://github.com/yourusername/container-census/issues
- **Next.js Documentation**: https://nextjs.org/docs
- **Chart.js Documentation**: https://www.chartjs.org/docs/

## License

This integration code is provided as part of the Container Census project.
See the main project LICENSE file for details.
