# OBJ Weapon Renderer (OBJ2PNG Studio)

A small Windows app that turns `.obj` 3D models (like CS2 weapons) into transparent PNG images. Drop in files or a whole folder and it renders them all.

<img width="1226" height="853" alt="OBJ2PNG Studio window" src="https://github.com/user-attachments/assets/1d197f08-c211-40bf-894a-32238792e637" />

## Features

- Drag and drop `.obj` files or folders into the queue.
- Renders several models at the same time, with progress and a cancel button.
- Image sizes up to 4000 x 2000, adjustable margins, and 2x anti-aliasing for smooth edges.
- 3-point lighting with brightness and contrast controls.
- Transparent PNG, or a dark background for previewing.
- Lays long models (like guns) out horizontally automatically, or pick the X, Y or Z view and mirror it.
- Keeps the model's own normals or calculates smooth ones.
- Remembers your settings.
- A simple command-line version for Linux and macOS (`main_linux.go`).

## Download

Get the exe from [Releases](https://github.com/Smokianlord/OBJ-Weapon-Renderer/releases).

## Build

See [BUILD_WINDOWS.md](BUILD_WINDOWS.md).
