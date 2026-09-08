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
// offsets/z-order, the per-state variant mapping, animation parameters, cursor
// look tracking, and hand/mouse tracking. Everything here is data-driven:
// editing the manifest changes behavior without recompiling.
type CharacterConfig struct {
	Skin       string                    `toml:"skin"`
	PartsOrder []string                  `toml:"parts_order"`
	Parts      map[string][]string       `toml:"parts"`
	Offsets    map[string][]Offset       `toml:"offsets"`
	Look       LookConfig                `toml:"look"`
	Tracking   TrackingConfig            `toml:"tracking"`
	Variants   map[string]map[string]int `toml:"variants"`
	Animations AnimationsConfig          `toml:"animations"`
}

// Offset is a part variant's position in the composition canvas (top-left
// origin). Offsets are per-variant: offsets["left"] has one entry per entry in
// parts["left"]; a shorter list falls back to the first entry.
type Offset struct {
	X int `toml:"x"`
	Y int `toml:"y"`
}

// LookConfig caps how far the head/eye parts track the cursor (px each axis).
type LookConfig struct {
	MaxX float64 `toml:"max_x"`
	MaxY float64 `toml:"max_y"`
}

// TrackingConfig makes parts follow the cursor (e.g. the hand on the mouse),
// clamped to max px per axis. Parts only move while the committed hand state
// is Mouse or Gaming.
type TrackingConfig struct {
	Parts []string `toml:"parts"`
	MaxX  float64  `toml:"max_x"`
	MaxY  float64  `toml:"max_y"`
}

// AnimationsConfig describes the built-in dynamic part behaviors. All are
// optional: leave a part empty (or amplitude 0) to disable a behavior.
type AnimationsConfig struct {
	Mouth MouthAnimConfig `toml:"mouth"`
	Blink BlinkAnimConfig `toml:"blink"`
	// Breathing subtly scales parts (body/head) about their centers.
	Breathing BreathingConfig `toml:"breathing"`
	// HandMoveDelaySecs debounces hand-layout changes: the hands only relocate
	// to a new state's pose after the raw state has been stable this long, so
	// the character doesn't flicker between mouse and keyboard during gaming.
	HandMoveDelaySecs float64 `toml:"hand_move_delay_secs"`
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

// BreathingConfig scales a set of parts about their centers.
type BreathingConfig struct {
	Parts      []string `toml:"parts"`
	PeriodSecs float64  `toml:"period_secs"`
	Amplitude  float64  `toml:"amplitude"`
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
