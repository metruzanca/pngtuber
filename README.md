# pngtuber

An open-source, cross-platform (Linux-first) **input-reactive desktop avatar**. It renders a
transparent, animated character that observes your computer activity (keyboard, mouse, microphone)
and mirrors your behavior. Capture it as a stream element in OBS.

**Status: Milestones 1–8 complete** — transparent undecorated window, global input pipeline, mic
capture + talking detection, the activity state machine, the rig character (state→variant parts,
elapsed-time animation, cursor tracking), a fully data-driven manifest, the asset data-dir pipeline
(scene-derived skins from a user-owned BitBuddy copy), and multi-skin selection. The character sits
at a desk with keyboard + mouse: body/head breathe, arms rest **down** by default and lift per
keystroke while typing, the mouse hand tracks the real mouse (even unfocused), gaming uses mouse +
keyboard together, and hand changes are debounced so the character doesn't flicker between mouse and
keyboard. See [IMPLEMENTATION_PLAN.md](IMPLEMENTATION_PLAN.md) for the roadmap and
[docs/bitbuddy-assets.md](docs/bitbuddy-assets.md) for installing real skins.

## Rig edit mode

Press **F2** to toggle a rig editor: drag any part with the mouse to reposition it, press **S**
to save mid-session, and **Esc** to exit — **exiting auto-saves** the offsets back into the loaded
`manifest.toml`. Parts listed together in `[character.groups]` (e.g. the eye + eyelid) drag as one
unit. All of a part's variants shift together. The head/eyes do not track the cursor by default
(see `[character.look] parts`).

The mouse hand and left keyboard hand are the same physical hand: only the actively-used one shows
(`[character.visibility]`), the mouse device stays visible at all times, and both poses are visible
in edit mode. The mouse hand follows the **real mouse** via the global input backend's relative
motion (see `[character.tracking] sensitivity`), so it moves even while the window is unfocused.

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

Everything is data-driven: a `manifest.toml` defines the rig (parts, scene-derived offsets,
z-order, per-state variant mapping, animations, cursor look) plus activity timings/thresholds.
Editing it changes behavior without recompiling.

Assets resolve in this order:

1. `PNGTUBER_ASSETS` env var or `--assets` flag
2. `$XDG_DATA_HOME/pngtuber/assets` (default `~/.local/share/pngtuber/assets`)
3. The committed placeholder (`assets/character/manifest.toml`), with a log line when used

Within the assets dir a skin lives at `skins/<skin>/manifest.toml` and is chosen with
`PNGTUBER_SKIN` or `--skin` (e.g. `pngtuber --skin alien_cat`). See
[docs/bitbuddy-assets.md](docs/bitbuddy-assets.md) for installing real BitBuddy skins; the window
auto-sizes to the skin's rig.