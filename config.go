package main

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

// FrameRange is a contiguous run of frames in the sprite sheet.
type FrameRange struct {
	First int `toml:"first"`
	Last  int `toml:"last"`
	FPS   int `toml:"fps"`
}

// Config is the data-driven character manifest (assets/character/manifest.toml).
type Config struct {
	Character CharacterConfig `toml:"character"`
	Activity  ActivityConfig  `toml:"activity"`
}

// CharacterConfig describes the sprite sheet and per-state frame mappings.
type CharacterConfig struct {
	Spritesheet string                `toml:"spritesheet"`
	FrameSize   FrameSize             `toml:"frame_size"`
	States      map[string]FrameRange `toml:"states"`
}

// FrameSize is the pixel size of a single cell in the sprite sheet.
type FrameSize struct {
	Width  int `toml:"width"`
	Height int `toml:"height"`
}

// ActivityConfig holds the state machine timings and mic threshold.
type ActivityConfig struct {
	IdleAfterSecs  float64 `toml:"idle_after_secs"`
	SleepAfterSecs float64 `toml:"sleep_after_secs"`
	MicThreshold   float64 `toml:"mic_threshold"`
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
