package external

import (
	"fmt"
	"strings"
)

// Permission represents a plugin permission
type Permission string

const (
	// Data access permissions
	PermContainersRead Permission = "containers:read"
	PermHostsRead      Permission = "hosts:read"
	PermStorageRead    Permission = "storage:read"
	PermStorageWrite   Permission = "storage:write"

	// API permissions
	PermAPIRoutes Permission = "api:routes"
	PermAPIEvents Permission = "api:events"

	// UI permissions
	PermUITab    Permission = "ui:tab"
	PermUIBadge  Permission = "ui:badge"
	PermUIEnrich Permission = "ui:enrich"
)

// PermissionChecker validates plugin permissions
type PermissionChecker struct {
	pluginID    string
	permissions map[Permission]bool
}

// NewPermissionChecker creates a permission checker for a plugin
func NewPermissionChecker(pluginID string, permissions []string) *PermissionChecker {
	permMap := make(map[Permission]bool)
	for _, p := range permissions {
		permMap[Permission(p)] = true
	}

	return &PermissionChecker{
		pluginID:    pluginID,
		permissions: permMap,
	}
}

// Check verifies if a plugin has a specific permission
func (p *PermissionChecker) Check(permission Permission) error {
	if !p.permissions[permission] {
		return fmt.Errorf("plugin %s does not have permission: %s", p.pluginID, permission)
	}
	return nil
}

// CheckAny verifies if a plugin has any of the specified permissions
func (p *PermissionChecker) CheckAny(permissions ...Permission) error {
	for _, perm := range permissions {
		if p.permissions[perm] {
			return nil
		}
	}
	return fmt.Errorf("plugin %s does not have any of the required permissions: %v", p.pluginID, permissions)
}

// Has returns true if the plugin has the specified permission
func (p *PermissionChecker) Has(permission Permission) bool {
	return p.permissions[permission]
}

// CanAccessEndpoint checks if a plugin can access a specific API endpoint
func (p *PermissionChecker) CanAccessEndpoint(method, path string) error {
	// Plugin routes require api:routes permission
	if strings.HasPrefix(path, "/api/p/") {
		return p.Check(PermAPIRoutes)
	}

	// Census API callbacks are checked individually in the gRPC server
	return nil
}

// ValidatePermissions validates that all requested permissions are known
func ValidatePermissions(permissions []string) error {
	knownPerms := map[string]bool{
		string(PermContainersRead): true,
		string(PermHostsRead):      true,
		string(PermStorageRead):    true,
		string(PermStorageWrite):   true,
		string(PermAPIRoutes):      true,
		string(PermAPIEvents):      true,
		string(PermUITab):          true,
		string(PermUIBadge):        true,
		string(PermUIEnrich):       true,
	}

	for _, p := range permissions {
		if !knownPerms[p] {
			return fmt.Errorf("unknown permission: %s", p)
		}
	}

	return nil
}

// GetPermissionDescription returns a human-readable description
func GetPermissionDescription(perm Permission) string {
	descriptions := map[Permission]string{
		PermContainersRead: "Read container data from all hosts",
		PermHostsRead:      "Read host configuration and metadata",
		PermStorageRead:    "Read plugin-specific data from storage",
		PermStorageWrite:   "Write plugin-specific data to storage",
		PermAPIRoutes:      "Register custom HTTP API routes",
		PermAPIEvents:      "Subscribe to system events",
		PermUITab:          "Add a custom UI tab to the interface",
		PermUIBadge:        "Display badges on container cards",
		PermUIEnrich:       "Add custom data to container details",
	}

	if desc, ok := descriptions[perm]; ok {
		return desc
	}
	return "Unknown permission"
}
