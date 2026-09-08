package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestResolveAssetsUserSkin verifies the flag/env/XDG resolution chain and the
// skins/<skin> layout for the M7 user data dir.
func TestResolveAssetsUserSkin(t *testing.T) {
	dir := t.TempDir()
	skinDir := filepath.Join(dir, "skins", "alien_cat")
	if err := os.MkdirAll(skinDir, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(skinDir, "manifest.toml")
	if err := os.WriteFile(manifest, []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}

	// --assets + --skin flags.
	res := ResolveAssets(dir, "alien_cat")
	if res.ManifestPath != manifest {
		t.Fatalf("ManifestPath = %q, want %q", res.ManifestPath, manifest)
	}
	if res.BaseDir != skinDir {
		t.Fatalf("BaseDir = %q, want %q", res.BaseDir, skinDir)
	}

	// Env fallback: PNGTUBER_ASSETS + PNGTUBER_SKIN.
	t.Setenv("PNGTUBER_ASSETS", dir)
	t.Setenv("PNGTUBER_SKIN", "alien_cat")
	res = ResolveAssets("", "")
	if res.ManifestPath != manifest {
		t.Fatalf("env ManifestPath = %q, want %q", res.ManifestPath, manifest)
	}

	// Skin missing -> falls back to placeholder.
	t.Setenv("PNGTUBER_SKIN", "nope")
	res = ResolveAssets("", "")
	if res.ManifestPath != placeholderManifest {
		t.Fatalf("missing-skin ManifestPath = %q, want placeholder %q", res.ManifestPath, placeholderManifest)
	}

	// Root manifest (no skin) is used when present.
	rootManifest := filepath.Join(dir, "manifest.toml")
	if err := os.WriteFile(rootManifest, []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PNGTUBER_SKIN", "")
	res = ResolveAssets("", "")
	if res.ManifestPath != rootManifest {
		t.Fatalf("root ManifestPath = %q, want %q", res.ManifestPath, rootManifest)
	}

	// Empty assets dir -> placeholder.
	t.Setenv("PNGTUBER_ASSETS", filepath.Join(t.TempDir(), "empty"))
	res = ResolveAssets("", "")
	if res.ManifestPath != placeholderManifest {
		t.Fatalf("empty-dir ManifestPath = %q, want placeholder", res.ManifestPath)
	}
}

func TestDefaultAssetsDir(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "/tmp/xdg")
	if got := defaultAssetsDir(); got != "/tmp/xdg/pngtuber/assets" {
		t.Errorf("with XDG_DATA_HOME = %q", got)
	}
	t.Setenv("XDG_DATA_HOME", "")
	home, _ := os.UserHomeDir()
	if got := defaultAssetsDir(); got != filepath.Join(home, ".local", "share", "pngtuber", "assets") {
		t.Errorf("without XDG_DATA_HOME = %q", got)
	}
}
