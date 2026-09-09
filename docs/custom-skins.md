# Creating your own skin

pngtuber is fully data-driven: a **skin** is just a folder with PNG part images
plus a `manifest.toml` that says how they compose and animate. No code, no
recompiling — drop the folder in the right place and run.

This page documents the whole manifest: every part, every table, and how the
pieces combine into behaviors. For installing/converting skins from a BitBuddy
copy you own, see [docs/bitbuddy-assets.md](bitbuddy-assets.md).

> **Assets note:** pngtuber ships **no** artwork. The binary contains a tiny
> generated placeholder rig so it runs out of the box, but any real character
> is something *you* provide. It just happens to also load BitBuddy-style
> rigs, which is documented separately.

## 1. Where skins live

Skins go under the assets directory, which resolves in this order:

1. `PNGTUBER_ASSETS` env var or `--assets` flag
2. `$XDG_DATA_HOME/pngtuber/assets` (default `~/.local/share/pngtuber/assets`)
3. The built-in placeholder (embedded in the binary)

Within the assets dir, a skin is a folder under `skins/`:

```
~/.local/share/pngtuber/assets/
└── skins/
    └── my_avatar/            # the skin name → --skin my_avatar
        ├── manifest.toml
        ├── body.png
        ├── head.png
        └── ...
```

Pick it with `pngtuber --skin my_avatar` (or `PNGTUBER_SKIN=my_avatar`). A
`manifest.toml` at the top of the assets dir itself is used when no skin is
selected. To start from the built-in placeholder, copy its files from the repo
(`assets/character/*`) into your skin folder and edit from there — that copy is
editable, the embedded one is read-only.

## 2. The parts

Every part is a **composited PNG** with an **(x, y) offset**. Parts have
**variants**: an ordered list of images, where **variant 0 is the
default/resting pose**. A special entry `"-"` marks a *hidden* variant (loaded
as nothing).

Conventional part names — anything not listed here still works, it just has no
built-in behavior:

| Part             | Purpose                                                    | Notes                                                    |
|------------------|------------------------------------------------------------|----------------------------------------------------------|
| `body`           | Torso base                                                 | Breathes; keep a cut where the mouth shows?              |
| `head`           | Head                                                       | Breathes                                                   |
| `eye`            | Visible eye                                                | Pairs with `eyelid` for blinking/closed-on-sleep          |
| `eyelid`         | `["-", lid.png]` = open, closed                            | `"-"` hidden when open, image when closed                 |
| `mouth`          | Talk frames                                                | Cycles while mic ≥ threshold                               |
| `left`/`right`   | Keyboard hands                                             | Down (0) and up (1) variants; `press` lifts them          |
| `mouse`          | Mouse hand                                                 | Down (0) and up (1) variants; stretches to cursor when used |
| `desk`           | Desk backdrop                                              | Draw it *above* the body to cover the bottom cut-off        |
| `keyboard`       | Keyboard                                                   | A prop; no behavior by default                            |
| `mouse_dev`      | Mouse device                                               | Follows the cursor when the mouse hand is active          |

`[character.parts_order]` sets z-order: **first = back, last = front**. The
unordered `[character.parts]`/`offsets` maps are drawn in this order, so list
every part you want drawn.

## 3. The manifest, section by section

A complete manifest with every recognized field:

```toml
[character]
skin = "my_avatar"
# z-order: back → front. Desk after body so it covers the body's bottom cut.
parts_order = ["body", "head", "eye", "eyelid", "mouth",
               "desk", "keyboard", "mouse_dev", "mouse", "right", "left"]

# name → ordered list of variant image files. "-" = hidden variant.
[character.parts]
desk      = ["desk.png"]
body      = ["body.png"]
head      = ["head.png"]
eye       = ["eye.png"]
eyelid    = ["-", "eyelid_closed.png"]   # open (hidden), closed
keyboard  = ["keyboard.png"]
mouse_dev = ["mouse_dev.png"]
left      = ["left_down.png", "left_up.png"]   # 0 = rest, 1 = up
right     = ["right_down.png", "right_up.png"]
mouse     = ["mouse_down.png", "mouse_up.png"]
mouth     = ["mouth_1.png", "mouth_2.png", "mouth_3.png"]

# One {x,y} TOP-LEFT offset per variant, same order as parts. A shorter list
# falls back to the first entry. Offsets are auto-normalized (rig shifts to
# (0,0)) and the window auto-sizes to the rig's bounding box.
[character.offsets]
desk      = [{ x = 0, y = 148 }]
body      = [{ x = 48, y = 100 }]
head      = [{ x = 64, y = 60 }]
eye       = [{ x = 84, y = 76 }]
eyelid    = [{ x = 84, y = 76 }, { x = 84, y = 76 }]
keyboard  = [{ x = 85, y = 78 }]
mouse_dev = [{ x = 126, y = 68 }]
left      = [{ x = 52, y = 110 }, { x = 52, y = 96 }]
right     = [{ x = 112, y = 110 }, { x = 112, y = 96 }]
mouse     = [{ x = 137, y = 106 }, { x = 137, y = 94 }]
mouth     = [{ x = 84, y = 100 }, { x = 84, y = 100 }, { x = 84, y = 100 }]

# Parts that drag as one unit in edit mode (eye + eyelid follow each other).
[character.groups]
eyes = ["eye", "eyelid"]

# The mouse hand and left keyboard hand are the SAME physical hand: only one
# shows at a time in normal mode (both visible in edit mode). Parts not listed
# (the mouse device, the right hand) always stay visible.
[character.visibility]
mouse_hand = ["mouse"]
keyboard_hand = ["left"]

# State → variant mapping. States: idle, mouse, typing, gaming, talking,
# sleep. Ant part not listed keeps variant 0 for that state.
[character.variants]
eyelid = { sleep = 1 }

# Dynamic behaviors — all optional (leave `part`/`parts` empty to disable).
[character.animations]
mouth            = { part = "mouth", fps = 10.0 }
press            = { parts = ["left", "right"], duration_secs = 0.1, variant = 1 }
blink            = { part = "eyelid", closed_variant = 1, interval_secs = 3.0, duration_secs = 0.15 }
breathing        = { parts = ["body", "head"], period_secs = 4.0, amplitude = 0.015 }
hand_move_delay_secs = 0.5   # how long the mouse hand stays after the mouse stills

# Parts that follow the REAL mouse (works unfocused, via global input),
# clamped to ±max per axis; motion is scaled by sensitivity.
[character.tracking]
parts = ["mouse", "mouse_dev"]
stretch = ["mouse"]          # top row pinned, stretches toward the cursor
max_x = 20
max_y = 12
sensitivity = 0.02

# Cursor look within the window: empty parts = eyes/head stay fixed.
[character.look]
parts = []
max_x = 10
max_y = 10

[activity]
idle_after_secs = 5.0        # quiet → idle state after this long
sleep_after_secs = 120.0     # idle → sleep (shuts the eyes)
mic_threshold = 0.08         # RMS level that counts as talking
mic_hysteresis = 0.5         # fraction of threshold to drop below to stop talking
gaming_window_secs = 1.0     # how recently both keys+mouse → "gaming"
```

### 3.1 `[activity]` states

The state machine (`[activity]`) picks the character's pose; the character
mirrors it per state via `[character.variants]`:

- **idle** — no input recently
- **typing** — keys
- **mouse** — mouse moving
- **gaming** — a key and a mouse event within `gaming_window_secs`
- **talking** — mic level ≥ `mic_threshold` (overrides the others while true)
- **sleep** — idle for `sleep_after_secs`

## 4. Wiring the pieces into behaviors

The magic is how the *pieces* combine — these recipes make the "input-reactive"
believe plausible.

**Hands that type:** give `left`/`right` a down (0) and up (1) variant, list
them in `press.parts`. Every keystroke taps the part to `press.variant` (1 =
hand up on the key) for `duration_secs`, alternating left/right.

**Hand layout switching (mouse vs keyboard):** the `left` keyboard hand is the
*same physical hand* as the `mouse` mouse hand. `visibility` decides which pose
shows: only `mouse_hand` parts while the mouse is active, only `keyboard_hand`
parts otherwise. The mouse hand appears the instant the real mouse moves and
returns to the keyboard `hand_move_delay_secs` after the mouse is still. Don't
list a part in both lists, and don't rely on other parts flipping this way.

**Mouse tracking:** `tracking.parts` follow the real mouse (relative deltas
from the global input backend — works even when the window is unfocused),
clamped to `max_x`/`max_y`. `stretch` parts keep their **top row fixed** and
stretch the bottom edge toward the cursor (read as the hand reaching); non-stretch
parts translate with the cursor. Drive the `mouse_dev` (as a translated part)
and `mouse` (as a stretched part) together for hand + cursor.

**Talking:** `mouth` cycles its variant frames at `fps` while the mic is ≥
`mic_threshold`. Lip-flap stops the instant audio drops below threshold (the
mouth follows the raw crossing, not the held talking state).

**Blinking + sleeping:** `blink` flips `eyelid` to `closed_variant` every
`interval_secs` for `duration_secs`; `eyelid = { sleep = 1 }` in `variants`
keeps the eyes closed through the whole sleep state.

**Breathing:** `breathing.parts` pulse subtle (amplitude ~0.01–0.03) so the
character looks alive while idle.

## 5. Minimal working skin, step by step

Start a folder `~/.local/share/pngtuber/assets/skins/demo/`. Copy the
placeholder PNGs from the repo's `assets/character/` as a base and trim the
manifest down — or draw your own 32×32 shapes and tweak offsets.

**Step 1 — a body only:**

```toml
[character]
skin = "demo"
parts_order = ["body"]
[character.parts]
body = ["body.png"]
[character.offsets]
body = [{ x = 0, y = 0 }]
[activity]
idle_after_secs = 5.0
sleep_after_secs = 120.0
```

Run `pngtuber --skin demo`: you get a window exactly the size of your body PNG.

**Step 2 — breathing:** add

```toml
[character.animations]
breathing = { parts = ["body"], period_secs = 4.0, amplitude = 0.02 }
```

**Step 3 — hand presses:** add `left_up.png`/`right_up.png`, register them,
and point `press` at them. Type — the hands should tap.

**Step 4 — talking:** add `mouth_1/2/3.png`, a `mouth` part with talking
frames, offsets in a row, and `mouth = { part = "mouth", fps = 10.0 }`. Talk —
the mouth cycles. Mic permission note below.

**Step 5 — blinking/sleeping:** `eyelid = ["-", "closed.png"]`, `blink` entry,
and `eyelid = { sleep = 1 }`. Wait for idle → the eyes close.

**Step 6 — mouse hand + desk:** append the rest of the placeholder's parts
(`desk`, `keyboard`, `mouse_dev`, `mouse`, …), `visibility`, `tracking`,
`variants`. Only needed if you want a whole desk scene.

## 6. Positioning parts: edit mode

Rather than hand-editing offsets (they're auto-normalized so the rig starts at
(0,0), but still fiddly), run with the skin and hit **F2**:

- **Left-drag** a part to move it; parts in a `[character.groups]` group (e.g.
  eye + eyelid) drag together. All of a part's variants shift together.
- **S** saves mid-session; **Esc** exits and **auto-saves** back into the
  skin's `manifest.toml` (byte-preserving — comments and other tables survive).
- The built-in placeholder is embedded and read-only: save logs a warning
  instead. Copy it into a real skin folder to make it editable.

## 7. Testing checklist

| You do                        | You should see                                      |
|-------------------------------|-----------------------------------------------------|
| Type                          | Hands toggle to the up variant per keystroke        |
| Move the mouse                | Mouse hand appears instantly, stretches to cursor   |
| Move the mouse, then stop     | Hand returns to keyboard after `hand_move_delay_secs` |
| Talk                          | Mouth cycles frames; stops instantly when quiet     |
| Wait `idle_after_secs`        | Body/head breathing continues, state idles          |
| Wait `sleep_after_secs`       | Eyelids close (sleep variant)                       |
| F2 + drag + Esc               | Offsets persist in the manifest                     |

## 8. BitBuddy-compatible skins

pngtuber also loads rigs extracted from a BitBuddy copy you own
(BitBuddy, © Saltfish): the binary scene→manifest tooling derives these exact
offsets/parts/animations automatically. That's covered in
[docs/bitbuddy-assets.md](bitbuddy-assets.md). Your own skins work identically —
they're just PNGs plus this manifest.