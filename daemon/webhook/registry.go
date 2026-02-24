package webhook

import "sync"

var (
	mu         sync.RWMutex
	extractors = make(map[string]Extractor)
)

func Register(name string, e Extractor) {
	mu.Lock()
	defer mu.Unlock()
	extractors[name] = e
}

func Get(name string) (Extractor, bool) {
	mu.RLock()
	defer mu.RUnlock()
	e, ok := extractors[name]
	return e, ok
}
