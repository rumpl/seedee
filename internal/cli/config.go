package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/rumpl/seedee/internal/core"
)

func loadPipelineConfig(path string) (*core.PipelineConfig, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolving config path: %w", err)
	}
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("config file not found: %s\n\nCreate a .seedee.yml file or use --config to specify a path", absPath)
	}
	cfg, err := core.LoadConfig(absPath)
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}
	return cfg, nil
}
