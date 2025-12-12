package graph

import (
	"fmt"

	"github.com/selfhosters-cc/container-census/internal/models"
)

type GraphNode struct {
	ID             string `json:"id"`
	Label          string `json:"label"`
	Type           string `json:"type"` // container, network, volume
	Status         string `json:"status,omitempty"`
	HostID         int64  `json:"host_id,omitempty"`
	ComposeProject string `json:"compose_project,omitempty"`
}

type GraphEdge struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Type  string `json:"type"` // network, volume, link, depends
	Label string `json:"label,omitempty"`
}

type ContainerGraph struct {
	Nodes []GraphNode `json:"nodes"`
	Edges []GraphEdge `json:"edges"`
}

// buildContainerGraph creates a graph representation of container relationships
func buildContainerGraph(containers []models.Container) *ContainerGraph {
	graph := &ContainerGraph{
		Nodes: make([]GraphNode, 0),
		Edges: make([]GraphEdge, 0),
	}

	networkMap := make(map[string]bool)
	volumeMap := make(map[string]bool)

	// First pass: add all container nodes and collect networks/volumes
	for _, container := range containers {
		// Add container node
		containerNodeID := fmt.Sprintf("container-%s-%d", container.ID, container.HostID)
		graph.Nodes = append(graph.Nodes, GraphNode{
			ID:             containerNodeID,
			Label:          container.Name,
			Type:           "container",
			Status:         container.Status,
			HostID:         container.HostID,
			ComposeProject: container.ComposeProject,
		})

		// Collect networks
		for _, networkName := range container.Networks {
			networkMap[networkName] = true
		}

		// Collect volumes
		for _, mount := range container.Volumes {
			if mount.Type == "volume" {
				volumeMap[mount.Name] = true
			}
		}
	}

	// Second pass: add network nodes
	for networkName := range networkMap {
		networkNodeID := fmt.Sprintf("network-%s", networkName)
		graph.Nodes = append(graph.Nodes, GraphNode{
			ID:    networkNodeID,
			Label: networkName,
			Type:  "network",
		})
	}

	// Third pass: add volume nodes
	for volumeName := range volumeMap {
		volumeNodeID := fmt.Sprintf("volume-%s", volumeName)
		graph.Nodes = append(graph.Nodes, GraphNode{
			ID:    volumeNodeID,
			Label: volumeName,
			Type:  "volume",
		})
	}

	// Fourth pass: create edges for network connections
	for _, container := range containers {
		containerNodeID := fmt.Sprintf("container-%s-%d", container.ID, container.HostID)

		for _, networkName := range container.Networks {
			networkNodeID := fmt.Sprintf("network-%s", networkName)
			graph.Edges = append(graph.Edges, GraphEdge{
				From:  containerNodeID,
				To:    networkNodeID,
				Type:  "network",
				Label: "",
			})
		}
	}

	// Fifth pass: create edges for volume mounts
	for _, container := range containers {
		containerNodeID := fmt.Sprintf("container-%s-%d", container.ID, container.HostID)

		for _, mount := range container.Volumes {
			if mount.Type == "volume" {
				volumeNodeID := fmt.Sprintf("volume-%s", mount.Name)
				graph.Edges = append(graph.Edges, GraphEdge{
					From:  containerNodeID,
					To:    volumeNodeID,
					Type:  "volume",
					Label: "",
				})
			}
		}
	}

	return graph
}
