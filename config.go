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
// offsets/z-order, the per-state variant mapping, animation parameters, and
// cursor look tracking. Everything here is data-driven: editing the manifest
// changes behavior without recompiling.
type CharacterConfig struct {
	Skin       string                    `toml:"skin"`
	PartsOrder []string                  `toml:"parts_order"`
	Parts      map[string][]string       `toml:"parts"`
	Offsets    map[string]Offset         `toml:"offsets"`
	Look       LookConfig                `toml:"look"`
	Variants   map[string]map[string]int `toml:"variants"`
	Animations AnimationsConfig          `toml:"animations"`
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

// AnimationsConfig describes the two built-in dynamic part behaviors. Both are
// optional: leave a part empty to disable the behavior.
type AnimationsConfig struct {
	Mouth MouthAnimConfig `toml:"mouth"`
	Blink BlinkAnimConfig `toml:"blink"`
}

// MouthAnimConfig makes a part cycle its variant sequence while Talking.
type MouthAnimConfig struct {
	Part string  `toml:"part"`
	FPS  float64 `toml:"fps"`
}

// BlinkAnimConfig makes a part flip to a "closed" variant periodically.
type BlinkAnimConfig struct {
	Part          string  `toml:"part"`
	ClosedVariant int     `toml:"closed_variant"`
	IntervalSecs  float64 `toml:"interval_secs"`
	DurationSecs  float64 `toml:"duration_secs"`
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
