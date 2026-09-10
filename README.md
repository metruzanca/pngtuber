# Simple PNGTuber

An open-source, Linux-first **input-reactive desktop avatar**. It renders a
transparent, animated character that watches your computer activity
(keyboard, mouse, microphone) and mirrors your behavior.

**v0.1.0** — releases: [Releases](https://github.com/metruzanca/pngtuber/releases)

## Features
- **Typing** the hands lift onto the keys per keystroke.
- **Mouse** the mouse hand appears the instant the real mouse moves (even
  when the window is unfocused) and stretches toward the cursor; it returns
  to the keyboard after you stop.
- **Gaming** keys + mouse together drive both hands.
- **Talking** the mouth flaps while the mic level is above the threshold,
  and stops the instant you go quiet.
- **Idle / sleep** the character breathes, then closes its eyes.
- Easy to capture via OBS

Build your own skins with the full manifest reference in
[docs/custom-skins.md](docs/custom-skins.md) or import one from [BitBuddy](docs/bitbuddy-assets.md) (Requires a copy of Bitbuddy).

## Install & run

Grab the latest binary from the
[Releases page](https://github.com/metruzanca/pngtuber/releases).

To use your own character, drop it into the user assets dir (`~/.local/share/pngtuber/assets/skins/<name>/`) and select it with the `--skin` flag or set the `PNGTUBER_SKIN` env var.
Asset folder can be changed with `--assets` flag or setting `PNGTUBER_ASSETS`.

## Troubleshooting

The prebuilt binary needs a few X11/GL runtime libraries. Most desktops already have these; if the app won't start, grab them for your distro:

<details>
<summary>Debian / Ubuntu</summary>
```
sudo apt install libx11-6 libxrandr2 libxinerama1 libxcursor1 libxi6 libxxf86vm1 libgl1 libglvnd0
```
</details>

<details>
<summary>Fedora</summary>
```
sudo dnf install libX11 libXrandr libXinerama libXcursor libXi libXxf86vm mesa-libGL libglvnd
```
</details>

<details>
<summary>Arch Linux</summary>
```
sudo pacman -S libx11 libxrandr libxinerama libxcursor libxi libxxf86vm mesa libglvnd
```
</details>

For **Keyboard/Mouse** observation reads `/dev/input/event*`. Ensure your user is in the `input` group, then re-log in. (`sudo usermod -aG input $USER`)

For **Voice-activity** detection needs a running PulseAudio or PipeWire (Pulse-compatible) server.

## Assets & licensing — please read

pngtuber is an **original implementation** of the input-reactive desktop-pet
concept. It does **not ship or bundle any BitBuddy artwork or assets**, those
belong to their respective owners (BitBuddy is © Saltfish) and are **not**
redistributed here. You provide your own artwork. Any `PNG` parts + a `manifest.toml` work.

BitBuddy does not support Linux and lots of effort is required to get it to work even under wine, so this project exists to fill in that blank. Theoretically, the author of Bitbuddy would be able to make it work underlinux with [godot-evdev](https://github.com/ShadowBlip/godot-evdev), but until then, this project exists. If Bitbuddy/saltfish has any issue with this app, feel free to contact me.
