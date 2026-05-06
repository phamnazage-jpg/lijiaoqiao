package platformadapter

import "strings"

type Registry struct {
	adapters map[string]PlatformAdapter
}

func NewRegistry(adapters ...PlatformAdapter) *Registry {
	r := &Registry{adapters: make(map[string]PlatformAdapter)}
	for _, adapter := range adapters {
		if adapter == nil {
			continue
		}
		r.Register(adapter)
	}
	return r
}

func (r *Registry) Register(adapter PlatformAdapter) {
	if r == nil || adapter == nil {
		return
	}
	if r.adapters == nil {
		r.adapters = make(map[string]PlatformAdapter)
	}
	key := strings.TrimSpace(strings.ToLower(adapter.Platform()))
	if key == "" {
		return
	}
	r.adapters[key] = adapter
}

func (r *Registry) Resolve(platform string) (PlatformAdapter, bool) {
	if r == nil {
		return nil, false
	}
	adapter, ok := r.adapters[strings.TrimSpace(strings.ToLower(platform))]
	return adapter, ok
}
