// Asset directory resolution (Milestone 7).
//
// Resolution order for the assets directory:
//
//	PNGTUBER_ASSETS env var  →  --assets flag  →  $XDG_DATA_HOME/pngtuber/assets
//	(Linux default ~/.local/share/pngtuber/assets).
//
// Within it, a skin's manifest is <assets>/skins/<skin>/manifest.toml when a
// skin is selected (PNGTUBER_SKIN or --skin), otherwise <assets>/manifest.toml.
// If no manifest exists, fall back to the committed placeholder
// (assets/character/manifest.toml) and log which directory was used, so
// `go run .` works out of the box. Real (copyrighted) skins are never shipped
// in the repo — they live in the user data dir.
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
)

// placeholderManifest is the committed test rig shipped in the repo.
const placeholderManifest = "assets/character/manifest.toml"

// AssetDirs is the resolved manifest path plus the directory part files
// resolve against (the manifest's own directory).
type AssetDirs struct {
	ManifestPath string
	BaseDir      string
	Source       string // description for the startup log
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
			log.Printf("assets: skin %q not found under %s; falling back to placeholder", skin, assetsDir)
		} else {
			log.Printf("assets: no manifest at %s; falling back to placeholder", candidate)
		}
	}
	log.Printf("assets: using placeholder at %s", placeholderManifest)
	return AssetDirs{ManifestPath: placeholderManifest, BaseDir: filepath.Dir(placeholderManifest), Source: "placeholder"}
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
