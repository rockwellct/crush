package ext

import (
	"fmt"
	"os"
	"sync"

	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
)

// YaegiProvider executes Go plugins via the Yaegi interpreter.
type YaegiProvider struct {
	mu sync.Mutex
}

// NewYaegiProvider creates a new YaegiProvider.
func NewYaegiProvider() *YaegiProvider {
	return &YaegiProvider{}
}

// LoadFile interprets a Go plugin file and executes its Init function.
func (p *YaegiProvider) LoadFile(path string, api API) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	i := interp.New(interp.Options{})
	if err := i.Use(stdlib.Symbols); err != nil {
		return fmt.Errorf("failed to load stdlib symbols: %w", err)
	}

	if err := ExportCustomSymbols(i); err != nil {
		return fmt.Errorf("failed to load custom extension symbols: %w", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read plugin file %s: %w", path, err)
	}

	if _, err := i.Eval(string(content)); err != nil {
		return fmt.Errorf("yaegi evaluation failed for %s: %w", path, err)
	}

	initVal, err := i.Eval("main.Init")
	if err != nil {
		// Init function is optional if the file just executed top-level code
		return nil
	}

	initFn, ok := initVal.Interface().(func(API))
	if !ok {
		return fmt.Errorf("main.Init in %s does not have expected signature func(ext.API)", path)
	}

	initFn(api)
	return nil
}
