// SPDX-License-Identifier: GPL-3.0-or-later

package app

import (
	"os"
	"path/filepath"
	"thaw/internal/config"
	"thaw/internal/sfconfig"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// LoadSnowflakeCLIConfig reads the Snowflake CLI configuration file (either from
// the custom path set by PickSnowflakeCLIConfigPath or the default location)
// and returns all named connection profiles together with the default one.
func (a *App) LoadSnowflakeCLIConfig() (sfconfig.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		return sfconfig.Config{}, err
	}
	scfg, err := sfconfig.Load(cfg.SnowflakeCLIConfigPath)
	if err != nil {
		return sfconfig.Config{}, err
	}
	return *scfg, nil
}

// GetSnowflakeCLIConfigPath returns the current path from which Snowflake CLI
// connection profiles are being loaded.
func (a *App) GetSnowflakeCLIConfigPath() (string, error) {
	cfg, err := config.Load()
	if err != nil {
		return "", err
	}
	if cfg.SnowflakeCLIConfigPath != "" {
		return cfg.SnowflakeCLIConfigPath, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", nil
	}
	return filepath.Join(home, ".snowflake", "config.toml"), nil
}

// sfconfigPath returns the Snowflake CLI config path from the app config.
func (a *App) sfconfigPath() (string, error) {
	cfg, err := config.Load()
	if err != nil {
		return "", err
	}
	return cfg.SnowflakeCLIConfigPath, nil
}

// SaveProfile creates or updates a named connection profile in the Snowflake
// CLI configuration file.
func (a *App) SaveProfile(profile sfconfig.Connection) error {
	p, err := a.sfconfigPath()
	if err != nil {
		return err
	}
	return sfconfig.SaveProfile(p, profile)
}

// RenameProfile renames an existing connection profile. If the old name was the
// default, the default is updated to the new name.
func (a *App) RenameProfile(oldName, newName string) error {
	p, err := a.sfconfigPath()
	if err != nil {
		return err
	}
	return sfconfig.RenameProfile(p, oldName, newName)
}

// DeleteProfile removes a named connection profile from the Snowflake CLI
// configuration file. If it was the default profile, the default is cleared.
func (a *App) DeleteProfile(name string) error {
	p, err := a.sfconfigPath()
	if err != nil {
		return err
	}
	return sfconfig.DeleteProfile(p, name)
}

// CloneProfile duplicates an existing profile under a new name.
func (a *App) CloneProfile(sourceName, newName string) error {
	p, err := a.sfconfigPath()
	if err != nil {
		return err
	}
	return sfconfig.CloneProfile(p, sourceName, newName)
}

// SetDefaultProfile sets the default_connection_name in the Snowflake CLI
// configuration file.
func (a *App) SetDefaultProfile(name string) error {
	p, err := a.sfconfigPath()
	if err != nil {
		return err
	}
	return sfconfig.SetDefaultProfile(p, name)
}

// ClearDefaultProfile removes the default_connection_name value in the
// Snowflake CLI configuration file.
func (a *App) ClearDefaultProfile() error {
	p, err := a.sfconfigPath()
	if err != nil {
		return err
	}
	return sfconfig.ClearDefaultProfile(p)
}

// PickSnowflakeCLIConfigPath opens a native file dialog to select a new
// Snowflake CLI configuration file. The selected path is persisted and
// used for all subsequent profile loads.
func (a *App) PickSnowflakeCLIConfigPath() (string, error) {
	cfg, err := config.Load()
	if err != nil {
		return "", err
	}

	initialDir := ""
	if cfg.SnowflakeCLIConfigPath != "" {
		initialDir = filepath.Dir(cfg.SnowflakeCLIConfigPath)
	} else {
		home, _ := os.UserHomeDir()
		if home != "" {
			initialDir = filepath.Join(home, ".snowflake")
		}
	}

	path, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title:            "Select Snowflake CLI Config",
		DefaultDirectory: initialDir,
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "Snowflake CLI Config (*.toml)", Pattern: "*.toml"},
			{DisplayName: "All Files", Pattern: "*.*"},
		},
	})
	if err != nil || path == "" {
		return "", err
	}

	// Re-load fresh inside Update rather than reusing the pre-dialog snapshot — the
	// file dialog can sit open for a while, during which another window may have
	// written config; a whole-struct Save of the stale cfg would revert that.
	if err := config.Update(func(c *config.AppConfig) error {
		c.SnowflakeCLIConfigPath = path
		return nil
	}); err != nil {
		return "", err
	}
	return path, nil
}
