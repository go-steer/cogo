package config

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Save writes cfg to path atomically (temp file in the same directory
// followed by rename). The output is human-edited friendly: stable key
// order, two-space indentation, trailing newline.
//
// Save does not validate; callers should run cfg.Validate() first.
func Save(path string, cfg *Config) error {
	if path == "" {
		return fmt.Errorf("config: empty target path")
	}
	if cfg == nil {
		return fmt.Errorf("config: nil config")
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("config: marshal: %w", err)
	}
	data = append(data, '\n')
	return atomicWrite(path, data, 0o644)
}

// atomicWrite mirrors internal/tools.atomicWrite but is duplicated here
// to keep the config package free of internal/tools imports (which
// would create a cycle).
func atomicWrite(path string, data []byte, mode fs.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".cogo-config-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
