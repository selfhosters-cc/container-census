# Container Census Plugin Development Guide

## Overview

Container Census supports external plugins via a gRPC-based architecture. Plugins run as separate processes and communicate with the Census server through gRPC, providing complete isolation and language independence.

## Table of Contents

1. [Architecture](#architecture)
2. [Plugin Types](#plugin-types)
3. [Getting Started](#getting-started)
4. [Plugin Manifest](#plugin-manifest)
5. [gRPC Protocol](#grpc-protocol)
6. [Permission System](#permission-system)
7. [Data Storage](#data-storage)
8. [Frontend Integration](#frontend-integration)
9. [Building and Releasing](#building-and-releasing)
10. [Example: Graph Visualizer Plugin](#example-graph-visualizer-plugin)

## Architecture

### Process Model

External plugins run as separate processes managed by the Census server:

```
┌─────────────────────────────────────────────────────────┐
│                  Census Server Process                   │
│                                                           │
│  ┌─────────────────┐        ┌──────────────────────┐   │
│  │ Plugin Manager   │───────▶│ External Plugin      │   │
│  │ - Supervisor     │        │ Supervisor           │   │
│  │ - Installer      │        │ - Process lifecycle  │   │
│  │ - Permissions    │        │ - Health monitoring  │   │
│  └─────────────────┘        │ - Log capture        │   │
│           │                  └──────────────────────┘   │
│           │                                              │
│           │                  ┌──────────────────────┐   │
│           └─────────────────▶│ Census API Server    │   │
│                              │ (gRPC)               │   │
│                              │ - GetContainers      │   │
│                              │ - GetHosts           │   │
│                              │ - PluginData         │   │
│                              └──────────────────────┘   │
└─────────────────────────────────────────────────────────┘
                       │
                       │ gRPC (localhost:50052)
                       │
         ┌─────────────┴─────────────┬──────────────────┐
         │                           │                  │
┌────────▼────────┐        ┌────────▼────────┐  ┌─────▼──────┐
│ Plugin Process  │        │ Plugin Process  │  │   ...      │
│ (Port 50100)    │        │ (Port 50101)    │  └────────────┘
│                 │        │                 │
│ - gRPC Server   │        │ - gRPC Server   │
│ - Plugin Logic  │        │ - Plugin Logic  │
│ - Frontend      │        │ - Frontend      │
└─────────────────┘        └─────────────────┘
```

### Communication Flow

1. **Census Server → Plugin**: Lifecycle management (Init, Start, Stop, Healthcheck)
2. **Plugin → Census Server**: Data queries (GetContainers, GetHosts, Storage operations)
3. **HTTP API → Plugin**: Route forwarding for plugin-specific endpoints
4. **Browser → Census Server**: Frontend asset serving from plugin directories

## Plugin Types

Plugins can provide multiple capabilities:

- **UI Tab**: Add a new tab to the interface
- **UI Badge**: Display badges on container cards
- **Container Enrichment**: Add custom data to container details
- **API Routes**: Register custom HTTP endpoints
- **Event Subscription**: Receive system events (container state changes, scans, etc.)

## Getting Started

### Prerequisites

- Go 1.23+ (for Go-based plugins)
- Protocol Buffers compiler (`protoc`)
- gRPC tooling for your language

### Plugin Structure

```
your-plugin/
├── plugin.yaml              # Plugin metadata
├── main.go                  # gRPC server implementation
├── proto/
│   └── plugin.proto         # Copy from Census repo
├── frontend/
│   ├── src/
│   │   ├── index.tsx       # Entry point
│   │   └── ...
│   ├── package.json
│   ├── webpack.config.js
│   └── dist/               # Built artifacts
├── .github/
│   └── workflows/
│       └── release.yml     # Automated builds
└── README.md
```

## Plugin Manifest

The `plugin.yaml` file describes your plugin:

```yaml
id: my-plugin
name: My Awesome Plugin
version: 1.0.0
description: A plugin that does awesome things
author: Your Name
repository: https://github.com/yourusername/census-plugin-my-plugin
min_census_version: "1.0.0"

# Requested permissions
permissions:
  - containers:read
  - hosts:read
  - storage:write
  - api:routes

# Frontend configuration
frontend:
  bundle_url: /plugin-assets/my-plugin/bundle.js
  css_url: /plugin-assets/my-plugin/bundle.css
  init_function: initMyPlugin

# UI tab configuration
tab:
  id: my-plugin-tab
  label: My Plugin
  icon: 🔌
  order: 20
```

### Manifest Fields

- **id**: Unique identifier (alphanumeric, hyphens, underscores)
- **name**: Human-readable display name
- **version**: Semantic version (major.minor.patch)
- **description**: Brief description of plugin functionality
- **author**: Plugin author/maintainer
- **repository**: GitHub repository URL (required for installation)
- **min_census_version**: Minimum compatible Census version
- **permissions**: Array of permission strings (see [Permission System](#permission-system))
- **frontend**: Frontend asset configuration
  - **bundle_url**: JavaScript bundle URL (relative to /api)
  - **css_url**: CSS file URL (optional)
  - **init_function**: JavaScript function name to initialize plugin
- **tab**: Tab configuration
  - **id**: Tab identifier
  - **label**: Display label
  - **icon**: Emoji or icon
  - **order**: Sort order (lower = earlier)

## gRPC Protocol

### Protocol Definition

Copy `internal/plugins/proto/plugin.proto` from the Census repository. The protocol defines two services:

#### 1. Plugin Service (Implemented by your plugin)

```protobuf
service Plugin {
  rpc Init(InitRequest) returns (InitResponse);
  rpc Start(StartRequest) returns (StartResponse);
  rpc Stop(StopRequest) returns (StopResponse);
  rpc Healthcheck(HealthcheckRequest) returns (HealthcheckResponse);
  rpc GetInfo(InfoRequest) returns (InfoResponse);
  rpc GetTab(TabRequest) returns (TabResponse);
  rpc HandleRoute(RouteRequest) returns (RouteResponse);
  rpc GetBadges(BadgesRequest) returns (BadgesResponse);
  rpc EnrichContainer(EnrichRequest) returns (EnrichResponse);
  rpc HandleEvent(EventRequest) returns (EventResponse);
  rpc GetSettings(SettingsRequest) returns (SettingsResponse);
  rpc UpdateSettings(UpdateSettingsRequest) returns (UpdateSettingsResponse);
}
```

#### 2. Census API Service (Provided by Census server)

```protobuf
service CensusAPI {
  rpc GetContainers(GetContainersRequest) returns (GetContainersResponse);
  rpc GetContainer(GetContainerRequest) returns (GetContainerResponse);
  rpc GetHosts(GetHostsRequest) returns (GetHostsResponse);
  rpc GetHost(GetHostRequest) returns (GetHostResponse);
  rpc GetPluginData(GetPluginDataRequest) returns (GetPluginDataResponse);
  rpc SetPluginData(SetPluginDataRequest) returns (SetPluginDataResponse);
  rpc DeletePluginData(DeletePluginDataRequest) returns (DeletePluginDataResponse);
  rpc Log(LogRequest) returns (LogResponse);
  rpc SendEvent(SendEventRequest) returns (SendEventResponse);
}
```

### Implementing a Plugin (Go Example)

```go
package main

import (
	"context"
	"log"
	"net"
	"os"

	"google.golang.org/grpc"
	pb "github.com/yourusername/your-plugin/proto"
)

type MyPlugin struct {
	pb.UnimplementedPluginServer
	pluginID    string
	censusAPI   pb.CensusAPIClient
}

func (p *MyPlugin) Init(ctx context.Context, req *pb.InitRequest) (*pb.InitResponse, error) {
	p.pluginID = req.PluginId

	// Connect to Census API
	conn, err := grpc.Dial(req.CensusApiAddress, grpc.WithInsecure())
	if err != nil {
		return &pb.InitResponse{Success: false, Error: err.Error()}, nil
	}
	p.censusAPI = pb.NewCensusAPIClient(conn)

	log.Printf("Plugin %s initialized", p.pluginID)
	return &pb.InitResponse{Success: true}, nil
}

func (p *MyPlugin) Healthcheck(ctx context.Context, req *pb.HealthcheckRequest) (*pb.HealthcheckResponse, error) {
	return &pb.HealthcheckResponse{
		Healthy: true,
		Status:  "running",
	}, nil
}

func (p *MyPlugin) HandleRoute(ctx context.Context, req *pb.RouteRequest) (*pb.RouteResponse, error) {
	if req.Path == "/data" && req.Method == "GET" {
		// Query containers from Census API
		containers, err := p.censusAPI.GetContainers(ctx, &pb.GetContainersRequest{
			PluginId:   p.pluginID,
			LatestOnly: true,
		})
		if err != nil {
			return &pb.RouteResponse{StatusCode: 500}, nil
		}

		// Process and return data
		data, _ := json.Marshal(containers)
		return &pb.RouteResponse{
			StatusCode: 200,
			Headers:    map[string]string{"Content-Type": "application/json"},
			Body:       data,
		}, nil
	}

	return &pb.RouteResponse{StatusCode: 404}, nil
}

func main() {
	// Read configuration from environment
	pluginID := os.Getenv("PLUGIN_ID")
	grpcPort := os.Getenv("GRPC_PORT")

	// Start gRPC server
	lis, _ := net.Listen("tcp", ":"+grpcPort)
	grpcServer := grpc.NewServer()
	pb.RegisterPluginServer(grpcServer, &MyPlugin{})

	log.Printf("Plugin %s listening on port %s", pluginID, grpcPort)
	grpcServer.Serve(lis)
}
```

## Permission System

### Available Permissions

| Permission | Description |
|------------|-------------|
| `containers:read` | Read container data from all hosts |
| `hosts:read` | Read host configuration and metadata |
| `storage:read` | Read plugin-specific data from storage |
| `storage:write` | Write plugin-specific data to storage |
| `api:routes` | Register custom HTTP API routes |
| `api:events` | Subscribe to system events |
| `ui:tab` | Add a custom UI tab to the interface |
| `ui:badge` | Display badges on container cards |
| `ui:enrich` | Add custom data to container details |

### Permission Enforcement

Permissions are enforced at the gRPC layer. If a plugin attempts to call a Census API method without the required permission, the call is rejected with an error.

Example:
```go
// Plugin calls GetContainers without containers:read permission
containers, err := censusAPI.GetContainers(ctx, req)
// Returns error: "plugin my-plugin does not have permission: containers:read"
```

## Data Storage

Plugins can store persistent data using the Census API's key-value storage:

```go
// Store data
_, err := p.censusAPI.SetPluginData(ctx, &pb.SetPluginDataRequest{
	PluginId: p.pluginID,
	Key:      "config",
	Value:    `{"refresh_interval": 60}`,
})

// Retrieve data
resp, err := p.censusAPI.GetPluginData(ctx, &pb.GetPluginDataRequest{
	PluginId: p.pluginID,
	Key:      "config",
})
if resp.Found {
	var config Config
	json.Unmarshal([]byte(resp.Value), &config)
}

// Delete data
_, err := p.censusAPI.DeletePluginData(ctx, &pb.DeletePluginDataRequest{
	PluginId: p.pluginID,
	Key:      "config",
})
```

Storage is automatically scoped to your plugin - you cannot access other plugins' data.

## Frontend Integration

### Frontend Structure

Your plugin's frontend should be built as a single JavaScript bundle:

```typescript
// frontend/src/index.tsx
import React from 'react';
import { createRoot } from 'react-dom/client';
import MyPluginView from './MyPluginView';

export function initMyPlugin(container: HTMLElement, sdk: any) {
  const root = createRoot(container);
  root.render(<MyPluginView sdk={sdk} />);
}

// Expose to global scope
declare global {
  interface Window {
    initMyPlugin: typeof initMyPlugin;
  }
}

window.initMyPlugin = initMyPlugin;
```

### Plugin SDK (Future)

The Census frontend will provide a Plugin SDK with these capabilities:

- **API Client**: Pre-configured HTTP client for plugin routes
- **Data Fetching**: Helper methods to fetch containers, hosts, etc.
- **UI Components**: Shared components for consistent styling
- **Toast Notifications**: Display success/error messages
- **Event Subscription**: Subscribe to frontend events

Example usage:
```typescript
function MyPluginView({ sdk }: { sdk: PluginSDK }) {
  const [data, setData] = useState(null);

  useEffect(() => {
    // Fetch data from plugin backend
    sdk.fetch('/data')
      .then(res => res.json())
      .then(setData);
  }, []);

  return <div>{/* Render data */}</div>;
}
```

### Building Frontend Assets

Use webpack or your preferred bundler:

```javascript
// webpack.config.js
module.exports = {
  entry: './src/index.tsx',
  output: {
    path: path.resolve(__dirname, 'dist'),
    filename: 'bundle.js',
  },
  module: {
    rules: [
      {
        test: /\.tsx?$/,
        use: 'ts-loader',
        exclude: /node_modules/,
      },
      {
        test: /\.css$/,
        use: ['style-loader', 'css-loader'],
      },
    ],
  },
};
```

Build command:
```bash
npm run build
# Outputs: dist/bundle.js, dist/bundle.css
```

## Building and Releasing

### Multi-Platform Builds

Build binaries for all supported platforms:

```bash
#!/bin/bash
# scripts/build.sh

VERSION=$(cat .version)

# Build backend
GOOS=linux GOARCH=amd64 go build -o dist/plugin-linux-amd64 .
GOOS=linux GOARCH=arm64 go build -o dist/plugin-linux-arm64 .
GOOS=darwin GOARCH=amd64 go build -o dist/plugin-darwin-amd64 .
GOOS=darwin GOARCH=arm64 go build -o dist/plugin-darwin-arm64 .

# Build frontend
cd frontend
npm install
npm run build
cp dist/* ../dist/frontend/
```

### GitHub Actions Release

Automate builds and releases:

```yaml
# .github/workflows/release.yml
name: Release

on:
  push:
    tags:
      - 'v*'

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.23'

      - name: Build binaries
        run: |
          chmod +x scripts/build.sh
          ./scripts/build.sh

      - name: Create Release
        uses: softprops/action-gh-release@v1
        with:
          files: |
            dist/plugin-linux-amd64
            dist/plugin-linux-arm64
            dist/plugin-darwin-amd64
            dist/plugin-darwin-arm64
            dist/frontend/bundle.js
            dist/frontend/bundle.css
          generate_release_notes: true
```

### Installation

Users install your plugin by providing the GitHub repository URL in the Census UI:

```
Settings → Plugins → Install Plugin
Repository URL: https://github.com/yourusername/census-plugin-my-plugin
```

The Census server will:
1. Fetch `plugin.yaml` from the repository
2. Download the appropriate binary for the server's platform
3. Download frontend assets
4. Store plugin metadata in the database
5. Start the plugin process
6. Mount API routes and serve frontend assets

## Example: Graph Visualizer Plugin

See the complete example plugin at: https://github.com/selfhosters-cc/cc-graph

This plugin demonstrates:
- Backend gRPC server implementation (Go)
- Querying container data via Census API
- Graph data processing and relationship mapping
- Frontend visualization with Cytoscape.js
- Color-coded containers (status and compose project modes)
- Interactive filtering (networks, volumes, stopped containers)
- Multiple layout algorithms (force-directed, hierarchical, etc.)
- Dynamic route handling
- Frontend asset bundling with webpack
- Multi-platform builds and GitHub releases

### Key Features Demonstrated

**Backend (`main.go`)**:
- gRPC Plugin service implementation
- Census API client for fetching container data
- Graph building algorithm (nodes and edges)
- Network and volume relationship detection
- Compose project grouping
- HTTP route handler for `/graph-data` endpoint

**Frontend (`frontend/src/index.js`)**:
- Vanilla JavaScript (no React) for minimal dependencies
- Cytoscape.js integration for graph visualization
- Dynamic color modes (status vs project)
- Interactive filters with real-time graph updates
- Custom legend generation
- SDK integration with fetch proxy and toast notifications

**Build Process**:
- Backend: Cross-compilation for linux/amd64, linux/arm64, darwin/amd64, darwin/arm64
- Frontend: Webpack bundle (single bundle.js file)
- GitHub Actions for automated releases
- Semantic versioning from `.version` file

## Troubleshooting

### Plugin Won't Start

Check logs via API:
```bash
curl -u admin:password http://localhost:8080/api/plugins/my-plugin/logs
```

Common issues:
- Missing permissions in plugin.yaml
- Binary not executable (chmod +x)
- Port conflict (check basePort in supervisor)
- gRPC connection failure (check CENSUS_API env var)

### Permission Denied Errors

Verify permissions in plugin.yaml match your API calls:
- Calling GetContainers? Need `containers:read`
- Calling SetPluginData? Need `storage:write`
- Registering routes? Need `api:routes`

### Frontend Not Loading

Check:
- Frontend assets exist in plugin directory: `/app/data/plugins/{plugin-id}/frontend/`
- Asset URLs in plugin.yaml match actual file names
- Browser console for JavaScript errors
- CSP headers aren't blocking execution

## Best Practices

1. **Error Handling**: Always return proper error responses in gRPC methods
2. **Healthchecks**: Implement robust health checking for automatic restart
3. **Logging**: Use the Census Log API for centralized logging
4. **Versioning**: Follow semantic versioning for compatibility
5. **Testing**: Test with multiple Census versions
6. **Security**: Validate all user input, prevent directory traversal
7. **Performance**: Cache data when possible, minimize API calls
8. **Documentation**: Include README with screenshots and usage examples

## Resources

- Census Repository: https://github.com/selfhosters-cc/container-census
- gRPC Documentation: https://grpc.io/docs/
- Protocol Buffers: https://developers.google.com/protocol-buffers
- Example Plugin: https://github.com/selfhosters-cc/container-census-plugin-graph
- Discord Community: [Link TBD]

## Contributing

Have questions or want to contribute to the plugin system? Open an issue or discussion in the Census repository!
