package graph

import (
	"context"
	_ "embed"
	"net/http"

	"github.com/selfhosters-cc/container-census/internal/plugins"
)

//go:embed frontend/bundle.js
var bundleJS []byte

//go:embed frontend/styles.css
var stylesCSS []byte

type GraphPlugin struct {
	deps plugins.PluginDependencies
}

func NewGraphPlugin() *GraphPlugin {
	return &GraphPlugin{}
}

func (p *GraphPlugin) Info() plugins.PluginInfo {
	return plugins.PluginInfo{
		ID:          "graph-visualizer",
		Name:        "Graph Visualizer",
		Version:     "1.0.0",
		Description: "Visualize container relationships using an interactive network graph",
		Author:      "Container Census Team",
		BuiltIn:     true,
	}
}

func (p *GraphPlugin) Init(ctx context.Context, deps plugins.PluginDependencies) error {
	p.deps = deps
	return nil
}

func (p *GraphPlugin) Start(ctx context.Context) error {
	p.deps.Logger.Info("Graph Visualizer plugin started")
	return nil
}

func (p *GraphPlugin) Stop(ctx context.Context) error {
	p.deps.Logger.Info("Graph Visualizer plugin stopped")
	return nil
}

func (p *GraphPlugin) Routes() []plugins.Route {
	return []plugins.Route{
		{
			Path:    "/graph-data",
			Method:  "GET",
			Handler: p.handleGraphData,
		},
		{
			Path:    "/bundle.js",
			Method:  "GET",
			Handler: p.serveBundleJS,
		},
		{
			Path:    "/styles.css",
			Method:  "GET",
			Handler: p.serveStylesCSS,
		},
	}
}

func (p *GraphPlugin) Tab() *plugins.TabDefinition {
	return &plugins.TabDefinition{
		ID:    "graph",
		Label: "Graph",
		Icon:  "🕸️",
		Order: 15,
	}
}

func (p *GraphPlugin) Badges() []plugins.BadgeProvider {
	return nil
}

func (p *GraphPlugin) ContainerEnricher() plugins.ContainerEnricher {
	return nil
}

func (p *GraphPlugin) Settings() *plugins.SettingsDefinition {
	return nil
}

func (p *GraphPlugin) NotificationChannelFactory() plugins.ChannelFactory {
	return nil
}

// serveBundleJS serves the embedded JavaScript bundle
func (p *GraphPlugin) serveBundleJS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/javascript")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Write(bundleJS)
}

// serveStylesCSS serves the embedded CSS file
func (p *GraphPlugin) serveStylesCSS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/css")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Write(stylesCSS)
}

// Register registers the graph plugin with the plugin manager
func Register(manager *plugins.Manager) {
	manager.RegisterBuiltIn("graph-visualizer", func() plugins.Plugin {
		return NewGraphPlugin()
	})
}
