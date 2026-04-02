package prompt

import "fmt"

// Version represents a prompt template version.
type Version struct {
	Name      string // e.g. "rca_system_v1"
	Active    bool
	Canary    []string // services using this version for canary
}

// Manager manages prompt versions and rollout.
type Manager struct {
	versions map[string]Version
	active   string
}

// NewManager creates a version manager.
func NewManager() *Manager {
	return &Manager{
		versions: make(map[string]Version),
	}
}

// GetVersion returns the prompt version for a given service.
// If the service is in a canary list, return the canary version.
// Otherwise, return the active version.
func (m *Manager) GetVersion(service string) (Version, error) {
	// Check canary first
	for _, v := range m.versions {
		for _, s := range v.Canary {
			if s == service {
				return v, nil
			}
		}
	}
	// Return active
	if v, ok := m.versions[m.active]; ok {
		return v, nil
	}
	return Version{}, fmt.Errorf("no active prompt version configured")
}
