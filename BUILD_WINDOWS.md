# Windows release build

The app icon is embedded in two ways:

1. `icon_windows.go` sets the runtime window/taskbar icon.
2. `embed_pe_icon.py` writes the RT_ICON + RT_GROUP_ICON PE resources so Windows Explorer shows the custom icon for the `.exe` itself.

Example from Linux with Go installed:

```bash
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-H=windowsgui -s -w" -o app_raw.exe .
python embed_pe_icon.py app_raw.exe obj2png_studio.ico OBJ2PNG_Studio_Pro_v1.0.0_Windows_x64.exe
```

Do not distribute `app_raw.exe`; it has no Explorer executable icon resource.
