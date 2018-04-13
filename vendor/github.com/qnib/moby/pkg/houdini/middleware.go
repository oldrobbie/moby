package houdini // import "github.com/docker/docker/pkg/houdini"

import (
	"net/http"
	"sync"

	"github.com/docker/docker/pkg/plugingetter"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
)

// Middleware uses a list of plugins to
// handle authorization in the API requests.
type Middleware struct {
	mu      sync.Mutex
	plugins []Plugin
}

// NewMiddleware creates a new Middleware
// with a slice of plugins names.
func NewMiddleware(names []string, pg plugingetter.PluginGetter) *Middleware {
	SetPluginGetter(pg)
	return &Middleware{
		plugins: newPlugins(names),
	}
}

func (m *Middleware) getHoudiniPlugins() []Plugin {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.plugins
}

// SetPlugins sets the plugin used for authorization
func (m *Middleware) SetPlugins(names []string) {
	m.mu.Lock()
	m.plugins = newPlugins(names)
	m.mu.Unlock()
}

// RemovePlugin removes a single plugin from this authz middleware chain
func (m *Middleware) RemovePlugin(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	plugins := m.plugins[:0]
	for _, authPlugin := range m.plugins {
		if authPlugin.Name() != name {
			plugins = append(plugins, authPlugin)
		}
	}
	m.plugins = plugins
}

// WrapHandler returns a new handler function wrapping the previous one in the request chain.
func (m *Middleware) WrapHandler(handler func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (*http.Request, error)) func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (*http.Request, error) {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (*http.Request, error) {
		plugins := m.getHoudiniPlugins()
		if len(plugins) == 0 {
			logrus.Debug("There are no houdini plugins in the chain")
			return r, nil
		}
		return r, nil
	}
}
