# OBJ2PNG Studio Pro

**Version:** 1.1.0

OBJ2PNG Studio Pro is a fast, native Windows desktop app for converting OBJ 3D models into transparent PNG renders, featuring multi-threaded batch rendering and Photoshop-friendly output.

<img width="1226" height="853" alt="image" src="https://github.com/user-attachments/assets/1d197f08-c211-40bf-894a-32238792e637" />

## Features

- **Native Windows Desktop UI**: Polished dark theme interface with zero external GUI framework dependencies.
- **Drag-and-Drop Workflow**: Drop individual `.obj` files or entire folders into the queue.
- **Parallel Batch Rendering**: Multi-threaded worker pool for fast batch processing with live progress tracking and cancellation support.
- **Quality & Resolution Presets**: Presets up to 4000 x 2000, customizable margins, and 2x SSAA supersampling with alpha-aware downsampling.
- **Studio Lighting & Tone Controls**: 3-point lighting (key, fill, rim) with specular highlight, adjustable brightness, and contrast controls.
- **Background Options**: Transparent PNG export or solid dark preview background.
- **Camera Auto-Framing**: Auto-aligns long models (such as weapons) horizontally, with support for manual axis views (Auto, X, Y, Z) and mirroring.
- **Normals Preservation**: Preserves original OBJ vertex normals or automatically computes smooth facet normals.
- **Settings Persistence**: User preferences are automatically remembered across sessions.
- **Cross-Platform CLI**: Lightweight command-line renderer for Linux and macOS (`main_linux.go`).

## Building

See [BUILD_WINDOWS.md](BUILD_WINDOWS.md) for instructions on building the standalone Windows executable and embedding the PE application icon.

## Releases

Download precompiled standalone binaries from the [Releases](https://github.com/Smokianlord/OBJ-Weapon-Renderer/releases) page.

