package main

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

// Config is the data-driven character manifest (assets/character/manifest.toml).
type Config struct {
	Character CharacterConfig `toml:"character"`
	Activity  ActivityConfig  `toml:"activity"`
}

// CharacterConfig describes the rig parts (composited PNGs), their composition
// offsets/z-order, and cursor look tracking.
type CharacterConfig struct {
	Skin       string              `toml:"skin"`
	PartsOrder []string            `toml:"parts_order"`
	Parts      map[string][]string `toml:"parts"`
	Offsets    map[string]Offset   `toml:"offsets"`
	Look       LookConfig          `toml:"look"`
}

// Offset is a part's position in the composition canvas (top-left origin).
type Offset struct {
	X int `toml:"x"`
	Y int `toml:"y"`
}

// LookConfig caps how far the head/eye parts track the cursor (px each axis).
type LookConfig struct {
	MaxX float64 `toml:"max_x"`
	MaxY float64 `toml:"max_y"`
}

// ActivityConfig holds the state machine timings and mic threshold.
type ActivityConfig struct {
	IdleAfterSecs  float64 `toml:"idle_after_secs"`
	SleepAfterSecs float64 `toml:"sleep_after_secs"`
	MicThreshold   float64 `toml:"mic_threshold"`
	// MicHysteresis is the fraction of the threshold the level must drop below
	// before a talking state is released (0..1, e.g. 0.5 = half threshold).
	MicHysteresis float64 `toml:"mic_hysteresis"`
	// GamingWindowSecs is how recently both a key and a mouse event must have
	// occurred for the state to be Gaming rather than Typing/Mouse.
	GamingWindowSecs float64 `toml:"gaming_window_secs"`
}

// LoadConfig reads the TOML manifest at path.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return &cfg, nil
}
