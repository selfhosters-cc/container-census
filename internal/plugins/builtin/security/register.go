package security

import "github.com/selfhosters-cc/container-census/internal/plugins"

// Register registers the security plugin with the plugin manager
func Register(manager *plugins.Manager) {
	manager.RegisterBuiltIn("security", func() plugins.Plugin {
		return NewSecurityPlugin()
	})
}
