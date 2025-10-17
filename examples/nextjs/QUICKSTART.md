# Quick Start Guide

Get Container Census telemetry data displaying in your Next.js site in under 5 minutes.

## Step 1: Configure Telemetry Collector

On your telemetry collector server, set an API key:

```bash
# Generate a secure API key
export STATS_API_KEY=$(openssl rand -hex 32)

# Add to your docker-compose.yml or environment
echo "STATS_API_KEY=$STATS_API_KEY"
```

Restart the telemetry collector to apply the change.

## Step 2: Install Dependencies

In your Next.js project:

```bash
npm install chart.js
```

## Step 3: Copy Integration Files

```bash
# From the container-census repository
cd /path/to/container-census

# Copy to your Next.js project
cp -r examples/nextjs/lib your-nextjs-app/
cp -r examples/nextjs/components your-nextjs-app/
cp examples/nextjs/app/telemetry/page.tsx your-nextjs-app/app/telemetry/
```

## Step 4: Configure Environment Variables

Create `.env.local` in your Next.js project root:

```bash
TELEMETRY_API_URL=https://your-telemetry-collector.example.com
TELEMETRY_API_KEY=your-api-key-from-step-1
```

## Step 5: Run Your App

```bash
npm run dev
```

Visit http://localhost:3000/telemetry to see your telemetry dashboard!

## Next Steps

- Customize the dashboard layout in `app/telemetry/page.tsx`
- Add more charts from the `components/` directory
- Style components to match your brand
- Read the full [README.md](./README.md) for advanced usage

## Troubleshooting

**"Cannot find module '@/lib/telemetry-api'"**
- Check that files are copied to the correct location
- Verify `tsconfig.json` has the `@/*` path alias configured

**"Invalid or missing API key" error**
- Verify `TELEMETRY_API_KEY` in `.env.local` matches `STATS_API_KEY` on collector
- Restart the Next.js dev server after changing `.env.local`

**Charts not rendering**
- Check browser console for errors
- Ensure `chart.js` is installed: `npm list chart.js`
- Verify components have `'use client'` directive

Still stuck? Check the main [README.md](./README.md) for detailed troubleshooting.
