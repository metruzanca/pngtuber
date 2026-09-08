#!/usr/bin/env bash
# Extract the BitBuddy .pck into a res://-style tree for scene2manifest.
#
# BitBuddy ships as a single Godot 4.6 pack (BitBuddy.pck, copyright Saltfish).
# godotpcktool unpacks it preserving the internal paths, including the
# .godot/exported/*.scn (binary coworker rig scenes), .godot/imported/*.ctex
# (WebP-wrapped textures) and the .remap/.import metadata Godot needs to
# resolve res:// paths when dumping a scene headless.
#
# Usage:
#   nix-shell -p godotpcktool --run './tools/extract_pck.sh BitBuddy.pck /tmp/pckout'
#   nix-shell -p godot --run \
#     'godot --headless --path /tmp/pckout --script tools/dump_scene.gd -- \
#       res://.godot/exported/133200997/<hash>-coworker_1036.scn > nodes.json'
set -euo pipefail

pck=${1:?usage: extract_pck.sh BitBuddy.pck <outdir>}
out=${2:?usage: extract_pck.sh BitBuddy.pck <outdir>}

godotpcktool -p "$pck" -a extract -o "$out"

echo "extracted to $out"
echo "next: dump a coworker scene with tools/dump_scene.gd, then run"
echo "  go run ./tools/scene2manifest -dump nodes.json -skin <skin> -out <manifest.toml>"