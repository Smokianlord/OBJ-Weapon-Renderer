# OBJ2PNG Studio Pro

**Version:** 1.0.0

OBJ2PNG Studio Pro is a native Windows desktop app for converting OBJ 3D models into transparent PNG renders, with batch rendering and Photoshop-friendly output.

## Features

- Native Windows desktop UI
- Drag-and-drop OBJ files and folders
- Batch rendering
- Sortable model queue with file name and location columns
- Persistent output folder
- Modern Windows file/folder pickers
- 4000 x 2000 max-quality preset
- 2x SSAA anti-aliasing
- Preserves OBJ vertex normals
- Transparent PNG output
- Auto-opens output folder after rendering
- Embedded application icon

## Build

See [BUILD_WINDOWS.md](BUILD_WINDOWS.md) for Windows build instructions.

## Source files

- `main_windows.go` - native Windows UI and app logic
- `renderer.go` - OBJ loading and PNG rendering
- `dialogs_windows.go` - Windows file/folder picker integration
- `icon_windows.go` - runtime icon handling
- `embed_pe_icon.py` - embeds the multi-size icon into the Windows executable
- `obj2png_studio.ico` - application icon
- `version.go` - release version
- `main_linux.go` - non-Windows fallback entry point
- `go.mod` - Go module definition

## Release

First public release: **v1.0.0**
