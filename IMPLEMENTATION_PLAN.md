# pngtuber Implementation Plan

> **Language/engine:** This plan is written for **Go + Ebitengine** (`github.com/hajimehoshi/ebiten/v2`),
> not Rust/Bevy. All sections below reflect the Go port. Progress is tracked by ticking off
> milestones in [§6 Milestones](#6-milestones).

pngtuber is an open-source, cross-platform (Linux-first) **input-reactive desktop avatar**.
It renders a transparent, animated character that observes computer activity and mirrors the
user's behavior:

```
Keyboard ──┐
Mouse ─────┼──> Activity/Behavior State ──> Character Animation
Microphone ┘
                 │
                 └──> Idle Timer ──> Sleep
```

Unlike a virtual pet, there is no gameplay. The computer's input activity *is* the source of
behavioral state. Unlike a classic PNGTuber, the avatar reacts not only to the microphone but to
global keyboard/mouse activity too. The window is intended to be captured by **OBS as a stream
element** (positioned/scaled inside OBS), not used as a click-through desktop overlay.

---

## 1. Goals & Non-Goals

### Goals
- Cross-platform desktop avatar, with **Linux as a first-class citizen** (X11 native, Wayland via XWayland).
- Transparent window that OBS can capture with full alpha (Game Capture + "Allow Transparency").
- Real-time reactive animation driven by a small activity state machine.
- Open source from day one; assets kept separate / replaceable.

### Current progress
- [x] Review + port plan to Go + Ebitengine (this doc)
- [x] Milestone 1: Bootstrap + transparent window
- [x] Milestone 2: Global input pipeline
- [x] Milestone 3: Mic capture
- [x] Milestone 4: Activity state machine
- [x] Milestone 5: Character animation
- [x] Milestone 6: Config manifest
- [x] Milestone 7: Asset data dir + first skin
- [x] Milestone 8: Asset integration (more skins)

### Non-Goals (current phase)
- Click-through window or always-on-top behavior (OBS positions the element).
- Conventional game / virtual-pet mechanics (feeding, stats, minigames).
- Voice *recognition* / STT. We only need voice *activity* (is the user talking) for now.
- Sending/injecting input; observe-only.
- A GUI settings panel in this phase (config is file-based).

---

## 2. Locked Decisions

| Topic | Decision | Rationale |
|---|---|---|
| Engine | Ebitengine v2.9 (`github.com/hajimehoshi/ebiten/v2`) | Simple `Game` loop fits a single-sprite app; `ScreenTransparent` support. Desktop backend is cgo+GLFW (needs X11/GL headers) |
| Window | Transparent, undecorated, normal window level | `RunGameOptions{ScreenTransparent:true}` + `SetWindowDecorated(false)`; OBS Game Capture handles placement; no click-through needed |
| Global input (Linux) | `github.com/grafov/evdev`, polling `/dev/input/event*` | Works on X11 **and** Wayland; maintained fork of golang-evdev |
| Global input (macOS/Windows) | Abstracted behind a `GlobalInputBackend`; initial backend Linux-only | Portable design without paying cross-platform costs up front |
| Mic input | `github.com/jfreymuth/pulse` (pure Go, no cgo) | Engine is pure Go → keep the whole stack cgo-free; speaks PulseAudio/PipeWire native protocol |
| Voice activity | Rolling RMS/peak amplitude → smoothed loudness + threshold | Lightweight; swap for real VAD later if needed |
| Animation | `ebiten.Image` sheet + `SubImage(rect)` per frame, driven by elapsed time | No ECS/atlas; manual frame index ticking at each state's fps |
| Cursor tracking | Track absolute cursor position (`ebiten.CursorPosition()`) | Native API; character can "look at" the mouse (resolves original open question) |
| Config format | TOML (`github.com/BurntSushi/toml`) | Readable, comments supported, Go-friendly; replaces Bevy RON |
| Assets | Not shipped yet | Animation config (manifest) is data-driven so real sheets drop in later |
| OBS integration | Game Capture with "Allow Transparency" (X11); PipeWire Video on Wayland | Wayland only reachable via XWayland, so PipeWire is the more reliable transparent path there |

**Wayland note:** Ebitengine dropped native Wayland in v2.6 (the `wayland` build tag was removed);
Wayland sessions run through **XWayland**. This changes the original "X11 *and* Wayland first-class"
claim — Wayland works but via the X compatibility layer, which affects OBS capture (see Risks §8).

---

## 3. Tech Stack

| Concern | Choice | Notes |
|---|---|---|
| Engine / render | `github.com/hajimehoshi/ebiten/v2` v2.9 | Desktop backend uses cgo + GLFW: needs X11 + OpenGL dev headers at build time, GL libs at runtime (see shell.nix on NixOS) |
| Linux input backend | `github.com/grafov/evdev` | Requires read access to `/dev/input/event*` (user in `input` group, or root) |
| Mic capture | `github.com/jfreymuth/pulse` | Pure Go PulseAudio/PipeWire client; no system libs beyond a running Pulse server |
| Config | `github.com/BurntSushi/toml` | Loads `manifest.toml`; data-driven states/thresholds |
| Thread comms | Goroutines + `sync/atomic` / channels | Input goroutine → activity counters; audio callback → atomic loudness |
| Animation | `ebiten.Image` + `SubImage(rect)`, elapsed-time frame stepping | Manual per-state frame controller |

**System deps:** X11 + OpenGL C dev headers (cgo/GLFW backend) and GL runtime libs. On NixOS use
`shell.nix`; on other distros install `libx11-dev`/`libgl-dev` equivalents. PulseAudio/PipeWire only
needed for mic (Milestone 3+). No `libasound2-dev`/`alsa-lib-devel` required.

---

## 4. Project Structure

```
.
├── go.mod / go.sum
├── main.go               # Game struct: window setup, RunGameWithOptions, wire subsystems
├── config.go             # Character manifest types + TOML loader (BurntSushi/toml)
├── assets.go             # Asset directory resolution (env/flag/XDG data dir)
├── activity.go           # ActivityState enum, signal merging, idle timer, transitions
├── input/
│   ├── input.go          # GlobalInputBackend trait + ActivitySignals counters
│   └── evdev_linux.go    # grafov/evdev polling (go:build linux)
├── mic.go                # jfreymuth/pulse record stream -> smoothed loudness
├── character.go          # rig composition, state->part controller, cursor visuals
├── window.go             # (optional) transparent window / RunGameOptions helper
├── tools/
│   └── scene2manifest/   # §5.8: extract coworker_*.scn + .import from BitBuddy.pck,
│                         #   parse scene -> generate manifest.toml parts/offsets
├── docs/
│   └── bitbuddy-assets.md # extracting BitBuddy assets (copyrighted, not distributed)
└── README.md             # /dev/input perms, PulseAudio, OBS capture, asset install

# User assets live OUTSIDE the repo (not committed; e.g. copyrighted game assets).
# Linux: $XDG_DATA_HOME/pngtuber/assets  (default ~/.local/share/pngtuber/assets)
~/.local/share/pngtuber/assets/
└── manifest.toml         # rig schema (states -> parts + offsets)
```

> There is no ECS. The `Game` struct implements `ebiten.Game` (`Update`, `Draw`, `Layout`) and holds
> plain fields for each subsystem; "events" (e.g. `StateChanged`) are delivered via channels or
> direct method calls rather than an event bus.

---

## 5. Core Components

### 5.1 Transparent window (`window.go` / `main.go`)

Ebitengine's transparent-window path (equivalent of Bevy's `transparent_window` example):

- `RunGameOptions{ScreenTransparent: true}` — equivalent of `ClearColor::NONE`.
- `ebiten.SetWindowDecorated(false)` — undecorated.
- `ebiten.SetWindowFloating(false)` — normal window level (OBS owns placement/size).
- `ebiten.SetRunnableOnUnfocused(true)` — **critical**: keeps the game loop updating while another
  app is focused (this is what enables global input reactivity).
- `ebiten.SetVsyncEnabled(false)` optional later for low-latency capture.
- Requires a compositor on X11 for alpha (same caveat as Bevy); document in README.

**Milestone risk:** validate transparency in OBS on both X11 and Wayland(XWayland) first, before
building the rest.

### 5.2 Global input (`input/`)

Observe-only input monitoring that works while other apps are focused.

- Spawn a dedicated goroutine at startup.
- Enumerate `/dev/input/event*`; select devices exposing keyboard keys (`EV_KEY`) or relative
  axes/buttons (`EV_REL`, mouse `BTN_*`).
- Classify each event into coarse signals and push into a bounded channel:
  - `KeyActivity` (key down, ignore auto-repeat)
  - `MouseActivity` (motion or button)
- The `Update` loop drains the channel each tick into `ActivitySignals` (event counters + timestamp
  of last activity).
- **Backend interface** `GlobalInputBackend` so macOS (event taps) / Windows (low-level hooks) can be
  added later without touching the activity layer.

**Failure handling:** if `/dev/input` is unreadable (not in `input` group), log a clear,
actionable warning and keep running in mic-only mode — never crash.

**Docs:** permissions (`sudo usermod -aG input $USER`, re-login) go in README.

### 5.3 Microphone (`mic.go`)

- Connect to the default source via `jfreymuth/pulse` (`pulse.NewClient` + `NewRecord` with an
  `Int16Writer` callback). Pure Go; speaks the PulseAudio/PipeWire native protocol.
- Compute per-buffer RMS in the record callback; low-pass smooth into an atomic (`atomic.Uint32`
  fixed point) to stay lock-free.
- The `Update` loop samples it into `MicLevel` (0..1 loudness + derived `is_talking` boolean against a
  configurable threshold with hysteresis).
- If no PulseAudio/PipeWire server is reachable, log a clear warning and run without mic — never crash.

### 5.4 Activity state machine (`activity.go`)

```go
type ActivityState int
const (
    Sleep   ActivityState = iota // long inactivity
    Idle                         // no activity, brief
    Typing                       // keyboard only
    Mouse                        // mouse only
    Gaming                       // keyboard + mouse within a short window
    Talking                      // mic above threshold
)
```

- Merge signals each tick in priority order (Talking > Gaming > Typing > Mouse > Idle > Sleep).
- Idle timer counts real time since last *any* activity.
  - `idle_after` → transition to `Idle`.
  - `sleep_after` → transition to `Sleep`.
  - Any activity resets the timer and wakes from sleep instantly.
- Emits a `StateChanged(ActivityState)` (channel / callback) so character/animation code reacts cleanly.
- All timings/thresholds live in `config.go`.

### 5.5 Character & animation (`character.go`)

- Load each rig part from the manifest as an `ebiten.Image`; compose parts each draw in manifest
  order/offsets (e.g. body → head → eye/eyelid → hands → mouth).
- A `CharacterState` tracks the current activity state; on `StateChanged` the controller switches
  which part variant to show (e.g. typing → `left`/`right` up frames; talking → `mouth` frames
  toggled by loudness; blink → `eyelid`).
- Part variants animate by **accumulated elapsed time** (not raw tick count) so the manifest's fps
  is accurate regardless of `SetTPS`.
- Cursor tracking: use `ebiten.CursorPosition()` to offset/angle the character toward the mouse
  (resolves the original "presence-only vs track cursor" question → track cursor).
- Rotation/blend between states is out of scope; switches are discrete for now (can soften later).
- Part offsets/z-order are **auto-derived from the skin's Godot scene** (see §5.8), not hand-tuned.

### 5.6 Config / manifest (`config.go` + TOML)

Assets arrive later, so the mapping is **data-driven**. TOML (replaces RON). The real BitBuddy
assets are **rig parts** (separate composited PNGs), not sprite sheets, so the schema describes
parts and their composition offsets:

```toml
[character]
skin = "alien_cat"

[character.parts]
body   = "alien_cat_body.png"
head   = "alien_cat_head.png"
eye    = "alien_cat_eye.png"
eyelid = "alien_cat_eyelid.png"
left   = ["alien_cat_left_up.png", "alien_cat_left_down.png"]
right  = ["alien_cat_right_up.png", "alien_cat_right_down.png"]
mouse  = ["alien_cat_mouse_up.png", "alien_cat_mouse_down.png"]
mouth  = ["mouth_1036_1.png", "mouth_1036_2.png", "mouth_1036_3.png"]

[character.offsets]   # auto-derived from the skin's Godot scene (see §5.8)
body   = { x = 0,  y = 0 }
head   = { x = 16, y = 8 }
eye    = { x = 0,  y = 0 }
...

[activity]
idle_after_secs = 5.0
sleep_after_secs = 120.0
mic_threshold = 0.08
```

Notes on the rig schema:
- Part lists encode **state variants** (e.g. `left = [up, down]` → typing toggles the hand; `mouth`
  frames → talking). The exact state→part mapping is finalized in the animation milestone (M5).
- Names vary per skin (`left_hand_up` vs `left_up`, optional `_no_mouth`/`_eye`); the manifest
  simply points at whatever files exist for the chosen skin.
- Until real assets exist, ship a tiny placeholder so the pipeline is testable in OBS.

### 5.7 Asset directory resolution (`assets.go`)

The app reads assets from a **user data directory**, not from the repo (BitBuddy assets are
copyrighted and must not be distributed):

- Resolve order: `PNGTUBER_ASSETS` env var → `--assets` flag → `$XDG_DATA_HOME/pngtuber/assets`
  (Linux default `~/.local/share/pngtuber/assets`).
- `manifest.toml` and all part images are loaded relative to that directory.
- If the dir has no manifest, fall back to the committed placeholder (`assets/character/`) and log
  which dir was used, so `go run .` works out of the box.
- `docs/bitbuddy-assets.md` documents extracting assets from a purchased BitBuddy copy and copying
  a skin into the data dir; users may supply their own rig parts instead.

### 5.8 Scene → manifest tooling (auto offsets)

The BitBuddy `.pck` (Godot 4.6 asset pack) is the source of truth: every skin has a `coworker_*.scn`
scene that defines the rig (per-part texture, position, scale, z-order). Offsets are **extracted
from the scene**, not guessed.

- Extract from the user's `BitBuddy.pck` with `godotpcktool` (committed helper script/tool):
  - `coworker_1036.scn` (alien_cat's rig scene)
  - `assets/skins/alien_cat/*.png.import` (ctex → png provenance)
- Parse the scene:
  - **Text path** (`.scn` as `[gd_scene` text): `github.com/atomicptr/godot-tscn-parser` → walk
    `Sprite2D` nodes → per-part `position`, `scale`, `z_index`, texture ref.
  - **Binary path** (`.scn` as `GDSC` binary): a small committed Godot project + headless `godot4`
    script that loads the scene and dumps node transforms to JSON.
- Map each node's texture ref to the PNG filename (via `.import`), then **auto-generate** the
  `[character.parts]` + `[character.offsets]` tables in `manifest.toml`.
- Result: a real skin installs into `~/.local/share/pngtuber/assets/` with correct composition, no
  hand-tuning.

---

## 6. Milestones

| # | Milestone | Deliverable | Exit criteria | Status |
|---|---|---|---|---|
| 1 | Bootstrap + transparent window | Ebitengine app, transparent undecorated window, test sprite, OBS guide stub | Window renders alpha correctly in OBS Game Capture (Allow Transparency) on X11 and Wayland(XWayland) | ✅ |
| 2 | Global input pipeline | evdev goroutine → `ActivitySignals`; activity logged to console | Keystrokes/mouse motion while another app is focused flip counters; no crash without `/dev/input` perms | ✅ |
| 3 | Mic capture | pulse → smoothed `MicLevel`, talking detection | Loudness moves with speech; threshold/hysteresis works; no audio server degrades gracefully | ✅ |
| 4 | Activity state machine | `ActivityState` merge + idle/sleep timers, `StateChanged` events | Correct transitions observed with combined input/mic scenarios | ✅ |
| 5 | Character animation | Rig composition, state→part controller, placeholder assets, cursor tracking | Character switches parts per state (hands/mouth), loops correctly, tracks cursor | ✅ |
| 6 | Config manifest | TOML-driven rig parts/offsets + activity thresholds | Editing `manifest.toml` changes behavior without recompiling | ✅ |
| 7 | Asset data dir + first skin | `assets.go` resolution, scene→manifest tool (§5.8), `docs/bitbuddy-assets.md`, one BitBuddy skin (alien_cat) installed locally | App loads a real skin from `~/.local/share/pngtuber/assets/`; composite renders correctly in OBS with offsets derived from `coworker_1036.scn` | ✅ |
| 8 | Asset integration (more skins) | Real rigs wired in for other skins as desired | Skin selection + per-skin parts/offsets work without code changes | ✅ |

### Suggested order rationale
Milestone 1 retires the single biggest technical risk (transparent capture) immediately.
Milestones 2–4 build the state machine with console/log feedback before any visual polish.
Milestone 5 adds the visuals on top of a working state machine. 6–7 make the whole thing asset-ready
(7 pulls a real copyrighted skin in locally without committing it). 8 generalizes to user-provided
skins.

---

## 7. Testing Strategy

- **Unit tests** where pure logic lives (`_test.go`):
  - Activity merge + priority rules.
  - Idle/sleep timer transitions (fake clock).
  - Frame-range looping (wraparound at `last`).
  - Cursor → state/angle mapping.
- **Integration smoke test** (manual, scripted):
  - Run app → type → move mouse → talk → go idle → sleep → wake. Verify each state visually.
- **OBS validation checklist** per platform (X11 / Wayland(XWayland) / macOS / Windows when available).
- CI later: `go vet ./...`, `go test ./...`, `gofmt -l .` on Linux.

---

## 8. Risks & Mitigations

| Risk | Impact | Mitigation |
|---|---|---|
| Ebitengine Wayland is XWayland-only | Medium | Use PipeWire Video capture on Wayland (more reliable transparency); Game Capture on X11. Validate in Milestone 1 |
| evdev requires `/dev/input` read perms | Medium | Clear README instructions (`input` group); graceful mic-only degradation + warning |
| Wayland window capture in OBS | Medium | Primary X11 path is Game Capture transparency; document PipeWire Video capture for Wayland |
| PulseAudio/PipeWire not running | Low | Log clear warning; degrade gracefully to input-only mode; README documents starting the daemon |
| `.scn` scene format is text or binary (unknown until extracted) | Medium | Detect on extraction; text → Go tscn parser, binary → headless Godot dump (§5.8) |
| Transparent window needs compositor (X11) | Low | Document compositor requirement in README (same caveat as Bevy) |
| cgo/GLFW build needs X11+OpenGL dev headers | Low | `shell.nix` on NixOS; documented `libx11-dev`/`libgl-dev` for other distros |
| BitBuddy assets are copyrighted (can't ship/commit) | Medium | Assets load from a user data dir (`~/.local/share/pngtuber/assets/`); repo ships docs + tooling only, never the art |

---

## 9. Open Questions

- **Sprite orientation / art scale**: 2D front-facing static character (like a PNGTuber rig frame)
  vs. animated directional? Affects whether "typing/gaming" needs multiple hand poses. Default
  assumption: single sprite sheet, rows per state, character reacts in place.
- **Lip sync fidelity**: is a talking/idle "talk" animation (mouth-open frames toggling at ~10 Hz
  while loud) enough, vs. sample-accurate mouth shapes? Default: loudness-gated talk animation.
- **Sleep visuals**: does "sleep" also hide/minimize the character (e.g., zzz particles) or simply
  switch frames? Default: switch frames only (window must remain capturable).

> Resolved in this revision: config format → **TOML**; mouse detail → **track cursor position**;
> mic → **jfreymuth/pulse (pure Go)**; engine → **Ebitengine v2.9**; animation assets → **rig parts
> (composited PNGs), not sprite sheets**; BitBuddy assets → **user data dir, not committed**;
> part offsets → **auto-derived from the skin's `coworker_*.scn` scene**, not hand-tuned (§5.8).

---

## 10. First Implementation Slice (after review)

Scaffolded the Go module with Ebitengine deps and implemented **Milestone 1** (done):

1. `go.mod`: `github.com/hajimehoshi/ebiten/v2 v2.9.11` (Go 1.26).
2. `main.go`: `Game` struct implementing `ebiten.Game`; transparent window via
   `RunGameOptions{ScreenTransparent:true}`, `SetWindowDecorated(false)`,
   `SetRunnableOnUnfocused(true)`.
3. `character.go` logic folded into `main.go` for M1: loads a placeholder sprite sheet and draws a
   test sprite (green circle on transparent), gently tracking the cursor.
4. `config.go` + `assets/character/manifest.toml`: manifest types + placeholder asset.
5. `shell.nix` (NixOS): X11/OpenGL dev headers + runtime GL libs for the cgo/GLFW build.
6. `README.md`: permissions + OBS capture steps.
7. Verified: `go build`, `go vet`, `gofmt` clean; app runs (smoke test via timeout, no errors).

**Note found during M1:** Ebitengine's Linux desktop backend is **cgo + GLFW**, not pure Go — it
needs X11/OpenGL dev headers to build and GL libs on the runtime loader path. The original
"pure Go, no cgo" assumption was corrected in §2/§3 and handled via `shell.nix`.

Next slice: **Milestone 2** — evdev goroutine → `ActivitySignals`, logged to console.

**Milestone 2 (done):** `input/` package — `GlobalInputBackend` trait + `ActivitySignals`
counters + bounded-channel `Drain` (`input.go`); Linux `grafov/evdev` backend that scans
`/dev/input/event*`, filters to keyboard/pointer devices, and classifies events into
`KeyActivity`/`MouseActivity` per device-goroutine (`evdev_linux.go`, `go:build linux`); non-Linux
no-op stub. `main.go` spawns the backend at startup, drains it into `ActivitySignals` each tick, and
logs one summary line per active tick plus on-screen counters. Without `/dev/input` perms it logs an
actionable `usermod -aG input` warning and keeps running (mic-only mode) instead of crashing.
Verified: `go test ./...`, `go vet`, `go build`, `gofmt` clean; smoke run showed real mouse events
filling counters while another app had focus. `github.com/grafov/evdev v1.0.0` added (uses cgo —
`linux/input.h` headers needed at build time, provided by `shell.nix`).

**Milestone 3 (done):** `mic.go` — `Mic` connects to the default PulseAudio/PipeWire source via
`jfreymuth/pulse` (`RecordLatency(0.05)` → ~50 ms buffers), computes per-buffer RMS, low-pass
smooths it (fast attack, slow release) into an `atomic.Uint32` 16.16 fixed-point, and exposes
`Level()` (0..1) + `Talking(threshold, hysteresis)` with release-at-half-threshold default. If no
audio server is reachable it logs a clear warning and returns nil (run without mic) — never crash.
`main.go` samples it each tick, logs talking transitions + a 2 s periodic level, and shows it on
screen; `mic_hysteresis` added to `manifest.toml`. Verified: `go test` (RMS/smoothing/fixed-point/
hysteresis), vet, build, gofmt clean; smoke run recorded the real default source (Razer Seiren V3)
and the level tracked ambient noise live.

**Milestone 4 (done):** `activity.go` — `ActivityState` enum (Sleep/Idle/Mouse/Typing/Gaming/
Talking) + `Activity` state machine. Each tick merges per-tick key/mouse deltas and the mic talking
flag in priority order (Talking > Gaming > Typing > Mouse > timers), holds the current pose inside
the active window, and runs idle/sleep timers against real time since the last *any* activity
(keyboard, mouse, or voice — talking keeps the avatar awake). Any activity wakes from sleep
instantly. Transitions are delivered via `SetOnChange` callback and logged. `gaming_window_secs`
added to `manifest.toml`. The clock is explicit (`Tick(now, key, mouse, micTalking)`) so timers are
unit-testable with a fake clock. Verified: priority, gaming window, idle→sleep→wake, talking reset,
and event emission unit tests; live smoke test showed typing → gaming → mouse transitions from real
keyboard/mouse input while another app had focus.

**Milestone 5 (done):** `character.go` — rig composition + state→part controller. The manifest
moved to the §5.6 rig schema: `parts` (name → variant image files), `offsets`, `parts_order`
(z-order), `[character.look]`. `LoadRig` loads every variant; `Character` picks a variant per
activity state (hands up/down for typing/mouse/gaming, eyelid closed on blink/sleep, mouth frames
animated while talking), animates variant sequences with `frameLoop` by **accumulated elapsed
time** (manifest fps honored regardless of TPS), blinks periodically, and the head/eye parts track
the cursor via a capped `lookOffset`. A generated placeholder rig (12 PNGs: body/head/eye/eyelid/
hands/mouth) is committed. Verified: `go test` (frame-loop wrap/reset, variant selection, blink
cycle, lookOffset), vet, build, gofmt clean; headless compositor rendered each state and pixel
checks confirmed per-state part switching (hands, eyelid, mouth size); live app starts, loads all
parts, and the window opens at 192×192. (`x11grab` unavailable in this ffmpeg build, so the visual
check was done via the headless compositor instead of a window capture.)

**Milestone 6 (done):** the last hardcoded state logic moved into the manifest. `config.go` gained
`[character.variants]` (part → state → variant index, keyed by activity name) and
`[character.animations]` (`mouth = {part, fps}`, `blink = {part, closed_variant, interval_secs,
duration_secs}`). `Character.variantIndex` now resolves the per-state mapping from TOML first, then
the dynamic mouth/blink behaviors (both optional). Parts, offsets, z-order, cursor look, animation
parameters, and activity thresholds are all data-driven: editing `manifest.toml` changes behavior
without recompiling. Verified by a dedicated test that rewrites a manifest and confirms the variant
selection and `idle_after_secs` change at runtime; full suite + vet + build + gofmt clean.

**Milestone 7 (done):** `assets.go` resolves the manifest from `PNGTUBER_ASSETS` → `--assets` →
`$XDG_DATA_HOME/pngtuber/assets`, with a skin subdir (`skins/<skin>/manifest.toml` via
`PNGTUBER_SKIN`/`--skin`) and a logged fallback to the committed placeholder. Offsets became
**per-variant** (`offsets[name] = [{x,y}, …]` matching the parts list) and parts gained a
`"-"` hidden variant (open eyelid), with `LoadRig` normalizing the rig to the origin and the window
auto-sizing to the rig's bounding box. Binary-path scene tooling committed per §5.8:
`tools/extract_pck.sh` (godotpcktool), `tools/dump_scene.gd` (headless Godot dump of the binary
`coworker_*.scn`), `tools/scene2manifest` (node dump JSON → manifest parts/offsets/variants/
animations, all scene-derived). `docs/bitbuddy-assets.md` documents the full install flow. The real
**alien_cat** skin (BitBuddy 4.6.1) was installed to
`~/.local/share/pngtuber/assets/skins/alien_cat/` and verified: the app loads it (`assets: loading
user data dir (…/skins/alien_cat/manifest.toml)`), the window auto-sizes to 154×149, and a
composite check confirmed opaque body/head at the scene-derived offsets. The placeholder still
works out of the box with a logged fallback. Copyrighted art stays out of the repo — only tooling +
docs are committed.

**Milestone 8 (done):** skin selection + per-skin parts/offsets work without code changes. A second
skin (**frog**, `coworker_1002.scn`) was run through the same pipeline and exposed one real naming
variation: it ships both `Body` (baked-in mouth) and `BodyNoMouth`, so `tools/scene2manifest` now
prefers `BodyNoMouth` when present (pngtuber draws the mouth as a separate part). The tool was
refactored into `buildManifest` and gained unit tests (node-name mapping incl. variations,
BodyNoMouth preference, offset normalization, z-order, blink/mouth animation emission). Both skins
install under `~/.local/share/pngtuber/assets/skins/` and load via `--skin`/`PNGTUBER_SKIN` with
no code changes: alien_cat → 154×149, frog → 165×128 (auto-sized windows). Full suite + vet +
build + gofmt clean. **All 8 milestones complete.**

**Post-milestone behavior pass:** reworked the character so it sits at a **desk**: the generated
manifests now include the shared desk environment (`desk_1.png`, `keyboard_1.png`, `mouse_1.png`
as static `desk`/`keyboard`/`mouse_dev` parts layered behind/under the character), the body+head
get a subtle **breathing** animation (`[character.animations] breathing`), the **mouse hand + mouse
device track the real cursor** while the mouse is active (`[character.tracking]`), typing presses
**both keyboard hands**, mouse-only uses the **mouse hand**, and gaming uses **mouse hand + a
keyboard hand** (`[character.variants]`). Hand-layout changes are **debounced** by
`hand_move_delay_secs = 1.0` (a raw/committed split in `Character`): the mouse hand only relocates
to the keyboard after the raw state has been stable ~1s, so brief mouse stalls during gaming never
make the character flicker between mouse and keyboard; sleep/wake and mouth stay instant. Layout
was reconstructed from the coworker scene's hand anchor points because the game positions its
desk/keyboard/mouse at runtime (sub-scenes dump to origin). Verified: unit tests (hand debounce,
flicker, instant sleep/wake, instant mouth, variant mapping, desk emission), headless per-state
composite pixel checks (desk renders, hands switch between idle/typing/mouse), and live runs —
alien_cat 320×251, frog 320×234.

**Edit mode + polish (follow-up):** added an interactive rig editor (`edit.go`). **F2** toggles it;
drag any part with the mouse to reposition (all its variants shift together); **S** saves the
offsets back into the loaded `manifest.toml` by rewriting only the `[character.offsets]` section
(byte-identical round-trip, comments preserved); **Esc** exits. The head/eyes no longer follow the
cursor by default — `[character.look]` gained a `parts` list (default empty; re-enable per part).
Hands now **tap up/down per keystroke** via a `press` animation
(`[character.animations] press = { parts=["left","right"], duration_secs=0.1, variant=1 }`,
alternating hands), instead of holding a static down pose while typing; the static
`left/right = {typing:1}` variant mapping was removed. Verified: unit tests (offset-save round-trip
incl. reload, hit-test, drag shift, press alternation + decay, look-parts config), headless
composite per state, and live edit-mode smoke tests (toggle, drag grab, save, exit).

**Follow-up fixes:** (1) **Exit edit mode now saves** — `setEditMode(false)` (Esc or toggling F2
off) persists the current offsets to the manifest automatically; S still works mid-session. (2) The
**mouse hand now tracks the real mouse even when the window is unfocused**: the input backend's
channel carries relative pointer motion (`Event{DX,DY}` from `EV_REL`), `ActivitySignals` accumulates
`MouseDX/MouseDY`, and `Character.UpdateMouse` integrates them into a clamped drifting offset
(`[character.tracking] sensitivity = 0.02`) that `Draw` applies to the tracking parts — replacing
the old `ebiten.CursorPosition()`-based tracking that only worked while focused. `Sensitivity` was
added to `TrackingConfig` and emitted by the tool. Verified: input delta accumulation, `eventFrom`
REL mapping, `UpdateMouse` integrate/clamp/decay/steady-reset, exit-save persistence tests; live run
confirmed global mouse motion flows while the window stayed unfocused.

**Arm inversion:** the arms and mouse hand were **inverted** so the **default/resting pose is
"down"** — variant 0 is now the `_down` sprite (hands resting on the desk/keyboard/mouse) and the
`_up` sprite is variant 1, used by the press animation to lift a hand per keystroke while typing.
`tools/scene2manifest` now maps `*HandUp` → ord 1 and `*HandDown` → ord 0 (offsets follow the scene
positions), and the static hand-state variant mapping was dropped (hands rest down in every state).
Skins + placeholder regenerated; tests updated (resting-down hand expectations, tool ord mapping).

**Edit-mode groups + hand visibility:** (1) `[character.groups]` (e.g. `eyes = ["eye","eyelid"]`)
makes parts drag together in edit mode — the eye and blinking eyelid can no longer be moved
independently; the editor snaps a whole group (`PartGroup`) and highlights every member. (2)
`[character.visibility]` treats the mouse hand and left keyboard hand as the **same physical hand**:
`mouse_hand = ["mouse"]` draws only while the committed state is Mouse/Gaming, `keyboard_hand =
["left"]` only otherwise — exactly one appears at a time in normal mode, while both are visible in
edit mode; the mouse device and right hand stay visible always. Tool emits both blocks; skins +
placeholder regenerated; unit tests (`partVisible` per state incl. edit mode, group drag) and
headless composite checks (left/mouse hand anchors differ between typing and mouse) verify.

**Hand stretch + faster hand switch:** the mouse hand no longer translates with the cursor — it
**stretches toward the mouse** instead: `[character.tracking] stretch = ["mouse"]` keeps the hand's
**top row of pixels fixed at the base position** (the base/wrist, due to perspective) and stretches
it so its **bottom edge reaches the tracked offset** — a horizontal skew + vertical scale composed
about the image top-center in canvas space (`stretchGeoM`, unit tested via `GeoM.Apply`: top row
pinned at every x, bottom row slides uniformly by (dx, dy)). The mouse device still translates with
the real cursor. The hand-layout debounce was **halved** (`hand_move_delay_secs` 1.0 → 0.5) so the
hands relocate to/from the keyboard in half the time. Tool emits both; skins + placeholder
regenerated.

**Asset note (added post-commit):** BitBuddy skins are **rig parts** (composited PNGs: body, head,
eye, eyelid, hands, mouth shapes). The game's `BitBuddy.pck` (Godot 4.6) contains a `coworker_*.scn`
scene per skin that defines exact part positions/z-order, so offsets are **auto-derived from the
scene** (§5.8) instead of hand-tuned. Assets are copyrighted (Saltfish) and load from a user data
dir (`~/.local/share/pngtuber/assets/`); the repo ships only docs + tooling + a placeholder. See
§5.6–5.8 and `docs/bitbuddy-assets.md`.
