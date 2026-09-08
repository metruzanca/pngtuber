# pngtuber

An open-source, cross-platform (Linux-first) **input-reactive desktop avatar**. It renders a
transparent, animated character that observes your computer activity (keyboard, mouse, microphone)
and mirrors your behavior. Capture it as a stream element in OBS.

**Status: Milestones 1–3 done** — transparent undecorated window with a test sprite, the global
input pipeline (evdev → activity counters), and mic capture (pulse → smoothed loudness + talking
detection with hysteresis). See [IMPLEMENTATION_PLAN.md](IMPLEMENTATION_PLAN.md) for the full
roadmap.

## Building

On NixOS, enter the dev shell (provides the X11/OpenGL headers + runtime GL libs Ebitengine needs):

```sh
nix-shell
go build ./...
go run .
```

On other distros, install the X11/OpenGL development headers (e.g. `libx11-dev`, `libgl-dev`) and
build with cgo enabled (default).

The window is **transparent, undecorated, and not floating**, sized 192×192. Press `Esc` to quit.

## OBS capture

The character window is meant to be captured as a stream element, not used as a desktop overlay.

### X11
1. Start a compositor (transparency needs one) if not already running (e.g. `picom`).
2. In OBS, add a **Window Capture** (XComposite) or **Game Capture** source pointed at the pngtuber
   window.
3. Enable **Allow Transparency** on the source.

### Wayland
Ebitengine runs via **XWayland** on Wayland sessions, so use OBS's **PipeWire Video** capture
window and select the pngtuber window. Transparency is preserved by the compositor.

## Permissions

Global keyboard/mouse observation (Milestone 2+) reads `/dev/input/event*`. Ensure your user is in
the `input` group and re-log in:

```sh
sudo usermod -aG input $USER
```

## Microphone

Voice-activity detection (Milestone 3+) uses `jfreymuth/pulse` (pure Go), which needs a running
PulseAudio or PipeWire (Pulse-compatible) server — the default on most desktop distros.

## Configuration

`assets/character/manifest.toml` maps activity states to sprite-sheet frame ranges and fps, plus
activity timings/thresholds. Editing it changes behavior without recompiling.