package graph

import (
	"encoding/json"
	"net/http"
)

// handleGraphData returns the container graph data as JSON
func (p *GraphPlugin) handleGraphData(w http.ResponseWriter, r *http.Request) {
	// Get all containers from the container provider
	containers := p.deps.Containers.GetContainers()

	// Build the graph
	graph := buildContainerGraph(containers)

	// Return as JSON
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(graph); err != nil {
		p.deps.Logger.Error("Failed to encode graph data: %v", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}
