# Binary-path scene dumper (Milestone 7 / §5.8).
#
# BitBuddy exports its coworker rig scenes as Godot BINARY PackedScenes
# (.scn, "RSRC" magic), which the text-tscn parser can't read. This script
# loads one with headless Godot and dumps every Sprite2D node's name, position,
# scale, z_index and texture as JSON for tools/scene2manifest.
#
# Usage (requires godot >= 4.x):
#   godot --headless --path <extracted_pck_dir> --script dump_scene.gd -- <scene_path> > nodes.json
#
# The --path must be the full extracted pck so Godot can resolve the scene's
# dependencies via the .remap/.import files. The scene path argument is
# relative to that project root.
extends SceneTree

func _init():
	var args := OS.get_cmdline_user_args()
	if args.is_empty():
		printerr("usage: -- <res://path/to/scene.scn>")
		quit(1)
		return
	var scene = load(args[0])
	if scene == null:
		printerr("ERR cannot load scene")
		quit(1)
		return
	var root = scene.instantiate()
	var out: Array = []
	_walk(root, out)
	print(JSON.stringify(out))
	root.free()
	quit(0)

func _walk(node: Node, out: Array) -> void:
	if node is Sprite2D:
		var entry := {
			"name": str(node.name),
			"type": node.get_class(),
			"position": [node.position.x, node.position.y],
			"scale": [node.scale.x, node.scale.y],
			"z_index": node.z_index,
			"flip_h": node.flip_h,
		}
		var tex: Texture2D = node.texture
		if tex != null:
			entry["texture"] = tex.resource_path
			entry["texture_size"] = [tex.get_width(), tex.get_height()]
		out.append(entry)
	for child in node.get_children():
		_walk(child, out)