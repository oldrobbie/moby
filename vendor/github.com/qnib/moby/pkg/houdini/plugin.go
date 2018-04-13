package houdini // import "github.com/docker/docker/pkg/houdini"

import (
	"sync"
	"net/http"

	"github.com/docker/docker/pkg/plugingetter"
	"github.com/docker/docker/pkg/plugins"
)

// Plugin allows third party plugins to manipulate requests
// in the context of docker API
type Plugin interface {
	// Name returns the registered plugin name
	Name() string

	// ManipulateRequest manipulates the request from the client to the daemon
	ManipulateRequest(*http.Request) (*http.Request, error)
}

// newPlugins constructs and initializes the authorization plugins based on plugin names
func newPlugins(names []string) []Plugin {
	plugins := []Plugin{}
	pluginsMap := make(map[string]struct{})
	for _, name := range names {
		if _, ok := pluginsMap[name]; ok {
			continue
		}
		pluginsMap[name] = struct{}{}
		plugins = append(plugins, newHoudiniPlugin(name))
	}
	return plugins
}

var getter plugingetter.PluginGetter

// SetPluginGetter sets the plugingetter
func SetPluginGetter(pg plugingetter.PluginGetter) {
	getter = pg
}

// GetPluginGetter gets the plugingetter
func GetPluginGetter() plugingetter.PluginGetter {
	return getter
}

// houdiniPlugin is an internal adapter to docker plugin system
type houdiniPlugin struct {
	initErr error
	plugin  *plugins.Client
	name    string
	once    sync.Once
}

func newHoudiniPlugin(name string) Plugin {
	return &houdiniPlugin{name: name}
}

func (a *houdiniPlugin) Name() string {
	return a.name
}

// Set the remote for an authz pluginv2
func (a *houdiniPlugin) SetName(remote string) {
	a.name = remote
}

func (a *houdiniPlugin) ManipulateRequest(r *http.Request) (*http.Request, error) {
	if err := a.initPlugin(); err != nil {
		return nil, err
	}
	// TODO: Do the magic
	return r, nil
}

// initPlugin initializes the authorization plugin if needed
func (a *houdiniPlugin) initPlugin() error {
	// Lazy loading of plugins
	a.once.Do(func() {
		if a.plugin == nil {
			var plugin plugingetter.CompatPlugin
			var e error

			if pg := GetPluginGetter(); pg != nil {
				plugin, e = pg.Get(a.name, HoudiniApiImplements, plugingetter.Lookup)
				a.SetName(plugin.Name())
			} else {
				plugin, e = plugins.Get(a.name, HoudiniApiImplements)
			}
			if e != nil {
				a.initErr = e
				return
			}
			a.plugin = plugin.Client()
		}
	})
	return a.initErr
}
