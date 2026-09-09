# pngtuber

An open-source, Linux-first **input-reactive desktop avatar**. It renders a
transparent, animated character that watches your computer activity
(keyboard, mouse, microphone) and mirrors your behavior — hands type when you
type, the mouse hand reaches toward your cursor, the mouth flaps when you
talk, and it falls asleep when you walk away. Capture it as a stream element
in OBS.

**v0.1.0** — releases: [Releases](https://github.com/metruzanca/pngtuber/releases)

## Assets & licensing — please read

pngtuber is an **original implementation** of the input-reactive desktop-pet
concept. It does **not ship or bundle any BitBuddy artwork or assets** — those
belong to their respective owners (BitBuddy is © Saltfish) and are **not**
redistributed here.

- You provide your own artwork. Any `PNG` parts + a `manifest.toml` work.
- Finished releases contain only your assets plus the tiny built-in test
  rig, so people download *your* character — never someone else's.
- **It so happens the rig format is compatible with BitBuddy's.** If you own
  a copy of BitBuddy, the included tooling can extract and convert its skins
  into this format — see [docs/bitbuddy-assets.md](docs/bitbuddy-assets.md)
  and use it at your own discretion (don't redistribute it).

Build your own skins with the full manifest reference in
[docs/custom-skins.md](docs/custom-skins.md).

## Install & run

Grab the latest binary from the
[Releases page](https://github.com/metruzanca/pngtuber/releases) (Linux
amd64; builds require X11/GL runtime libraries as on any Ebitengine app), or
build from source:

```sh
nix-shell          # NixOS: provides X11/GL headers + runtime GL libs
go build ./...
go run .
```

First run shows the **built-in test rig** so the app works out of the box.
To use your own character, drop it into the user assets dir and select it:

```sh
# ~/.local/share/pngtuber/assets/skins/<name>/
pngtuber --skin my_avatar
```

Run `pngtuber --version` to check the build; `--debug` adds an in-window
status overlay (state + input/mic counters) and verbose console logs.

### How the character behaves

- **Typing** — the hands lift onto the keys per keystroke.
- **Mouse** — the mouse hand appears the instant the real mouse moves (even
  when the window is unfocused) and stretches toward the cursor; it returns
  to the keyboard after you stop.
- **Gaming** — keys + mouse together drive both hands.
- **Talking** — the mouth flaps while the mic level is above the threshold,
  and stops the instant you go quiet.
- **Idle / sleep** — the character breathes, then closes its eyes.

### Flags

| Flag                    | Meaning                                        |
|-------------------------|------------------------------------------------|
| `--skin <name>`         | Skin under `<assets>/skins/` (env `PNGTUBER_SKIN`) |
| `--assets <dir>`        | Assets directory (env `PNGTUBER_ASSETS`)       |
| `--debug`               | Status overlay + verbose logging               |
| `--version`             | Print the build version and exit               |

Press **F2** for the rig editor (drag parts, `S` save, `Esc` exit — auto-saves
offsets back into the manifest). Both hands are drawn in edit mode.

## OBS capture

The character window is meant to be captured as a stream element, not used as
a desktop overlay.

### X11
1. Start a compositor (transparency needs one) if not already running (`picom`).
2. In OBS, add a **Window Capture (XComposite)** or **Game Capture** source
   pointed at the pngtuber window.
3. Enable **Allow Transparency** on the source.

### Wayland
Ebitengine runs via **XWayland** on Wayland sessions, so use OBS's **PipeWire
Video** capture window and select the pngtuber window. Transparency is
preserved by the compositor.

## Permissions

Global keyboard/mouse observation reads `/dev/input/event*`. Ensure your user
is in the `input` group, then re-log in:

```sh
sudo usermod -aG input $USER
```

## Microphone

Voice-activity detection uses `jfreymuth/pulse` (pure Go), which needs a
running PulseAudio or PipeWire (Pulse-compatible) server — the default on most
desktop distros. The mic merely measures loudness; no audio leaves your machine.

## Configuration & data files

Everything is data-driven: a `manifest.toml` defines the rig (parts,
scene-derived offsets, z-order, per-state variant mapping, animations, cursor
look) plus activity timings/thresholds. Editing it changes behavior without
recompiling.

Assets resolve in this order:

1. `PNGTUBER_ASSETS` env var or `--assets` flag
2. `$XDG_DATA_HOME/pngtuber/assets` (default `~/.local/share/pngtuber/assets`)
3. The built-in embedded placeholder (annotated as such at startup)

## Releases & development

- **Releases** are cut from `v*` git tags by the
  [GitHub Actions workflow](.github/workflows/release.yml) driving
  [goreleaser](.goreleaser.yaml) (Linux, archive + checksums). Tag and push:
  `git tag v0.1.0 && git push origin v0.1.0`.
- Development shell: `nix-shell` (see `shell.nix`); the CI-equivalent checks are
  `go test ./... && go vet ./... && go build ./... && gofmt -l .`.
- Roadmap: [IMPLEMENTATION_PLAN.md](IMPLEMENTATION_PLAN.md).

## Documentation

- [docs/custom-skins.md](docs/custom-skins.md) — create your own skin: the
  full manifest reference, part conventions, and a worked example.
- [docs/bitbuddy-assets.md](docs/bitbuddy-assets.md) — extract + convert a
  BitBuddy copy you own into a pngtuber skin.