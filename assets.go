// Asset directory resolution (Milestone 7).
//
// Resolution order for the assets directory:
//
//	PNGTUBER_ASSETS env var  →  --assets flag  →  $XDG_DATA_HOME/pngtuber/assets
//	(Linux default ~/.local/share/pngtuber/assets).
//
// Within it, a skin's manifest is <assets>/skins/<skin>/manifest.toml when a
// skin is selected (PNGTUBER_SKIN or --skin), otherwise <assets>/manifest.toml.
// If no manifest exists, fall back to the built-in placeholder
// (assets/character/* embedded into the binary) and log which source was
// used, so `go run .` and released binaries work out of the box. Real
// (copyrighted) skins are never shipped in the repo — they live in the user
// data dir.
package main

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/charmbracelet/log"
)

//go:embed assets/character/*
var placeholderFS embed.FS

// placeholderManifest is the embedded test rig's manifest path (relative to
// placeholderFS).
const placeholderManifest = "assets/character/manifest.toml"

// AssetDirs is the resolved manifest path plus the directory part files
// resolve against (the manifest's own directory). FS is non-nil when loading
// the embedded placeholder, in which case BaseDir/ManifestPath are relative
// to FS; a nil FS means normal OS filesystem paths.
type AssetDirs struct {
	ManifestPath string
	BaseDir      string
	Source       string // description for the startup log
	FS           fs.FS  // non-nil => load from the embedded placeholder
}

// ResolveAssets determines the manifest to load from the environment/flags,
// falling back to the committed placeholder.
func ResolveAssets(assetsFlag, skinFlag string) AssetDirs {
	assetsDir := assetsFlag
	if assetsDir == "" {
		assetsDir = os.Getenv("PNGTUBER_ASSETS")
	}
	if assetsDir == "" {
		assetsDir = defaultAssetsDir()
	}

	skin := skinFlag
	if skin == "" {
		skin = os.Getenv("PNGTUBER_SKIN")
	}

	if assetsDir != "" {
		var candidate string
		if skin != "" {
			candidate = filepath.Join(assetsDir, "skins", skin, "manifest.toml")
		} else {
			candidate = filepath.Join(assetsDir, "manifest.toml")
		}
		if fileExists(candidate) {
			return AssetDirs{
				ManifestPath: candidate,
				BaseDir:      filepath.Dir(candidate),
				Source:       fmt.Sprintf("user data dir (%s)", candidate),
			}
		}
		if skin != "" {
			log.Warnf("assets: skin %q not found under %s; falling back to placeholder", skin, assetsDir)
		} else {
			log.Warnf("assets: no manifest at %s; falling back to placeholder", candidate)
		}
	}
	log.Infof("assets: using built-in placeholder")
	return AssetDirs{ManifestPath: placeholderManifest, BaseDir: "assets/character", Source: "built-in placeholder", FS: placeholderFS}
}

// defaultAssetsDir returns $XDG_DATA_HOME/pngtuber/assets (or
// ~/.local/share/pngtuber/assets on Linux).
func defaultAssetsDir() string {
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, "pngtuber", "assets")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".local", "share", "pngtuber", "assets")
}

func fileExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}

// LoadConfigFS reads and parses a manifest from an fs.FS (used for the
// embedded placeholder). Path is relative to the root of the FS.
func LoadConfigFS(fsys fs.FS, path string) (*Config, error) {
	data, err := fs.ReadFile(fsys, path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return &cfg, nil
}

// LoadConfigFrom loads the manifest from res, reading from the embedded
// placeholder FS when res.FS is non-nil, otherwise from the OS filesystem.
func LoadConfigFrom(res AssetDirs) (*Config, error) {
	if res.FS != nil {
		return LoadConfigFS(res.FS, res.ManifestPath)
	}
	return LoadConfig(res.ManifestPath)
}
