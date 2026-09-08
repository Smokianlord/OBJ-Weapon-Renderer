//go:build windows

package main

import (
	_ "embed"
	"encoding/binary"
	"runtime"
	"syscall"
	"unsafe"
)

//go:embed obj2png_studio.ico
var embeddedAppICO []byte

var pCreateIconFromResourceEx = user32.NewProc("CreateIconFromResourceEx")

const (
	WM_SETICON      = 0x0080
	ICON_SMALL      = 0
	ICON_BIG        = 1
	LR_DEFAULTCOLOR = 0x0000
)

type icoEntry struct {
	width, height int
	size, offset  uint32
}

func bestICOEntry(want int) []byte {
	b := embeddedAppICO
	if len(b) < 6 || binary.LittleEndian.Uint16(b[0:2]) != 0 || binary.LittleEndian.Uint16(b[2:4]) != 1 {
		return nil
	}
	n := int(binary.LittleEndian.Uint16(b[4:6]))
	if len(b) < 6+n*16 {
		return nil
	}
	bestScore := int(^uint(0) >> 1)
	var best []byte
	for i := 0; i < n; i++ {
		e := b[6+i*16 : 6+(i+1)*16]
		ww, hh := int(e[0]), int(e[1])
		if ww == 0 {
			ww = 256
		}
		if hh == 0 {
			hh = 256
		}
		sz := binary.LittleEndian.Uint32(e[8:12])
		off := binary.LittleEndian.Uint32(e[12:16])
		if int(off+sz) > len(b) || sz == 0 {
			continue
		}
		d := ww - want
		if d < 0 {
			d = -d
		}
		d2 := hh - want
		if d2 < 0 {
			d2 = -d2
		}
		score := d + d2
		if score < bestScore {
			bestScore = score
			best = b[int(off):int(off+sz)]
		}
	}
	return best
}

func loadEmbeddedAppIcon(size int) syscall.Handle {
	data := bestICOEntry(size)
	if len(data) == 0 {
		return 0
	}
	h, _, _ := pCreateIconFromResourceEx.Call(
		uintptr(unsafe.Pointer(&data[0])),
		uintptr(len(data)),
		1,
		0x00030000,
		uintptr(size), uintptr(size),
		LR_DEFAULTCOLOR,
	)
	runtime.KeepAlive(data)
	return syscall.Handle(h)
}

func applyEmbeddedWindowIcons(hwnd syscall.Handle) {
	big := loadEmbeddedAppIcon(64)
	small := loadEmbeddedAppIcon(20)
	if big != 0 {
		send(hwnd, WM_SETICON, ICON_BIG, uintptr(big))
	}
	if small != 0 {
		send(hwnd, WM_SETICON, ICON_SMALL, uintptr(small))
	}
}
