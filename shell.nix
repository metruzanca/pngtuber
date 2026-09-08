# pngtuber NixOS development shell.
# Provides the C toolchain + X11/OpenGL headers that Ebitengine's Linux desktop
# (cgo + GLFW) backend needs at build time.
#
# Usage:
#   nix-shell
#   go build ./...

{ pkgs ? import <nixpkgs> {} }:

pkgs.mkShell {
  buildInputs = [
    # C toolchain (cgo). Note: the system Go toolchain (go 1.26.6, matching
    # go.mod) is used as-is; providing pkgs.go here caused a version mismatch
    # with the host toolchain's build cache.
    pkgs.pkg-config

    # X11 development headers required by Ebitengine's GLFW backend
    pkgs.libx11.dev
    pkgs.libxrandr.dev
    pkgs.libxinerama
    pkgs.libxcursor
    pkgs.libxi
    pkgs.libxxf86vm

    # OpenGL headers and library
    pkgs.libGL.dev
    pkgs.libGLU
    pkgs.libglvnd.dev

    # Runtime GL libraries (libGL / libGLESv2) so the app can start from the shell
    pkgs.libGL
    pkgs.libglvnd
  ];

  shellHook = ''
    # Runtime GL libraries (dlopened by Ebitengine at startup) must be on
    # LD_LIBRARY_PATH, not just NIX_LDFLAGS.
    export LD_LIBRARY_PATH="${pkgs.libGL}/lib:${pkgs.libglvnd}/lib:${pkgs.libGLU}/lib:$LD_LIBRARY_PATH"
    echo "pngtuber dev shell. Building with:"
    go version
    echo "CGO_ENABLED=$CGO_ENABLED"
  '';
}
