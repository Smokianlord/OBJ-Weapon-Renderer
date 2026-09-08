//go:build windows

package main

import (
	"fmt"
	"runtime"
	"strings"
	"syscall"
	"unsafe"
)

type GUID struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

type COMDLG_FILTERSPEC struct {
	PszName *uint16
	PszSpec *uint16
}

var (
	ole32Dlg                     = syscall.NewLazyDLL("ole32.dll")
	shell32Dlg                   = syscall.NewLazyDLL("shell32.dll")
	pCoInitializeEx              = ole32Dlg.NewProc("CoInitializeEx")
	pCoUninitialize              = ole32Dlg.NewProc("CoUninitialize")
	pCoCreateInstance            = ole32Dlg.NewProc("CoCreateInstance")
	pCoTaskMemFree               = ole32Dlg.NewProc("CoTaskMemFree")
	pSHCreateItemFromParsingName = shell32Dlg.NewProc("SHCreateItemFromParsingName")
)

var (
	CLSID_FileOpenDialog = GUID{0xDC1C5A9C, 0xE88A, 0x4DDE, [8]byte{0xA5, 0xA1, 0x60, 0xF8, 0x2A, 0x20, 0xAE, 0xF7}}
	IID_IFileOpenDialog  = GUID{0xD57C7288, 0xD4AD, 0x4768, [8]byte{0xBE, 0x02, 0x9D, 0x96, 0x95, 0x32, 0xD9, 0x60}}
	IID_IShellItem       = GUID{0x43826D1E, 0xE718, 0x42EE, [8]byte{0xBC, 0x55, 0xA1, 0xE2, 0x61, 0xC3, 0x7B, 0xFE}}
)

const (
	COINIT_APARTMENTTHREADED = 0x2
	CLSCTX_INPROC_SERVER     = 0x1
	FOS_PICKFOLDERS          = 0x00000020
	FOS_FORCEFILESYSTEM      = 0x00000040
	FOS_ALLOWMULTISELECT     = 0x00000200
	FOS_PATHMUSTEXIST        = 0x00000800
	FOS_FILEMUSTEXIST        = 0x00001000
	SIGDN_FILESYSPATH        = 0x80058000
	HRESULT_CANCELLED        = uint32(0x800704C7)
)

func failedHR(r uintptr) bool { return int32(uint32(r)) < 0 }
func hrErr(r uintptr, what string) error {
	return fmt.Errorf("%s failed (HRESULT 0x%08X)", what, uint32(r))
}

func methodAt(obj unsafe.Pointer, index uintptr) uintptr {
	vtbl := *(*unsafe.Pointer)(obj)
	return *(*uintptr)(unsafe.Pointer(uintptr(vtbl) + index*unsafe.Sizeof(uintptr(0))))
}

func comCall(obj unsafe.Pointer, index uintptr, args ...uintptr) uintptr {
	a := make([]uintptr, 0, 1+len(args))
	a = append(a, uintptr(obj))
	a = append(a, args...)
	r, _, _ := syscall.SyscallN(methodAt(obj, index), a...)
	return r
}

func comRelease(obj unsafe.Pointer) {
	if obj != nil {
		comCall(obj, 2)
	}
}

func utf16PtrString(p *uint16) string {
	if p == nil {
		return ""
	}
	a := (*[1 << 20]uint16)(unsafe.Pointer(p))
	n := 0
	for n < len(a) && a[n] != 0 {
		n++
	}
	return syscall.UTF16ToString(a[:n])
}

func shellItemPath(item unsafe.Pointer) (string, error) {
	var psz *uint16
	hr := comCall(item, 5, SIGDN_FILESYSPATH, uintptr(unsafe.Pointer(&psz)))
	if failedHR(hr) {
		return "", hrErr(hr, "IShellItem.GetDisplayName")
	}
	if psz == nil {
		return "", nil
	}
	s := utf16PtrString(psz)
	pCoTaskMemFree.Call(uintptr(unsafe.Pointer(psz)))
	return s, nil
}

func setDialogInitialFolder(dlg unsafe.Pointer, initial string) {
	if strings.TrimSpace(initial) == "" {
		return
	}
	p, err := syscall.UTF16PtrFromString(initial)
	if err != nil {
		return
	}
	var item unsafe.Pointer
	hr, _, _ := pSHCreateItemFromParsingName.Call(
		uintptr(unsafe.Pointer(p)), 0,
		uintptr(unsafe.Pointer(&IID_IShellItem)),
		uintptr(unsafe.Pointer(&item)),
	)
	runtime.KeepAlive(p)
	if failedHR(hr) || item == nil {
		return
	}
	comCall(dlg, 12, uintptr(item)) // IFileDialog::SetFolder
	comRelease(item)
}

func withFileOpenDialog(fn func(unsafe.Pointer) ([]string, error)) ([]string, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	hr, _, _ := pCoInitializeEx.Call(0, COINIT_APARTMENTTHREADED)
	if failedHR(hr) && uint32(hr) != 0x80010106 { // RPC_E_CHANGED_MODE
		return nil, hrErr(hr, "CoInitializeEx")
	}
	if !failedHR(hr) {
		defer pCoUninitialize.Call()
	}
	var dlg unsafe.Pointer
	r, _, _ := pCoCreateInstance.Call(
		uintptr(unsafe.Pointer(&CLSID_FileOpenDialog)), 0, CLSCTX_INPROC_SERVER,
		uintptr(unsafe.Pointer(&IID_IFileOpenDialog)), uintptr(unsafe.Pointer(&dlg)),
	)
	if failedHR(r) || dlg == nil {
		return nil, hrErr(r, "CoCreateInstance(FileOpenDialog)")
	}
	defer comRelease(dlg)
	return fn(dlg)
}

func showDialog(dlg unsafe.Pointer, owner syscall.Handle) (bool, error) {
	hr := comCall(dlg, 3, uintptr(owner)) // IModalWindow::Show
	if uint32(hr) == HRESULT_CANCELLED {
		return false, nil
	}
	if failedHR(hr) {
		return false, hrErr(hr, "IFileDialog.Show")
	}
	return true, nil
}

func modernPickOBJFiles(owner syscall.Handle) ([]string, error) {
	return withFileOpenDialog(func(dlg unsafe.Pointer) ([]string, error) {
		name, _ := syscall.UTF16PtrFromString("Wavefront OBJ (*.obj)")
		spec, _ := syscall.UTF16PtrFromString("*.obj")
		filter := COMDLG_FILTERSPEC{name, spec}
		hr := comCall(dlg, 4, 1, uintptr(unsafe.Pointer(&filter))) // SetFileTypes
		runtime.KeepAlive(name)
		runtime.KeepAlive(spec)
		if failedHR(hr) {
			return nil, hrErr(hr, "SetFileTypes")
		}
		opts := uintptr(FOS_FORCEFILESYSTEM | FOS_FILEMUSTEXIST | FOS_PATHMUSTEXIST | FOS_ALLOWMULTISELECT)
		if hr = comCall(dlg, 9, opts); failedHR(hr) {
			return nil, hrErr(hr, "SetOptions")
		}
		title, _ := syscall.UTF16PtrFromString("Add OBJ models")
		comCall(dlg, 17, uintptr(unsafe.Pointer(title)))
		runtime.KeepAlive(title)
		ok, err := showDialog(dlg, owner)
		if err != nil || !ok {
			return nil, err
		}
		var arr unsafe.Pointer
		hr = comCall(dlg, 27, uintptr(unsafe.Pointer(&arr))) // IFileOpenDialog::GetResults
		if failedHR(hr) || arr == nil {
			return nil, hrErr(hr, "GetResults")
		}
		defer comRelease(arr)
		var count uint32
		hr = comCall(arr, 7, uintptr(unsafe.Pointer(&count))) // IShellItemArray::GetCount
		if failedHR(hr) {
			return nil, hrErr(hr, "GetCount")
		}
		out := make([]string, 0, count)
		for i := uint32(0); i < count; i++ {
			var item unsafe.Pointer
			hr = comCall(arr, 8, uintptr(i), uintptr(unsafe.Pointer(&item))) // GetItemAt
			if failedHR(hr) || item == nil {
				continue
			}
			p, e := shellItemPath(item)
			comRelease(item)
			if e == nil && p != "" {
				out = append(out, p)
			}
		}
		return out, nil
	})
}

func modernPickFolder(owner syscall.Handle, titleText, initial string) ([]string, error) {
	return withFileOpenDialog(func(dlg unsafe.Pointer) ([]string, error) {
		opts := uintptr(FOS_PICKFOLDERS | FOS_FORCEFILESYSTEM | FOS_PATHMUSTEXIST)
		hr := comCall(dlg, 9, opts)
		if failedHR(hr) {
			return nil, hrErr(hr, "SetOptions")
		}
		title, _ := syscall.UTF16PtrFromString(titleText)
		comCall(dlg, 17, uintptr(unsafe.Pointer(title)))
		runtime.KeepAlive(title)
		setDialogInitialFolder(dlg, initial)
		ok, err := showDialog(dlg, owner)
		if err != nil || !ok {
			return nil, err
		}
		var item unsafe.Pointer
		hr = comCall(dlg, 20, uintptr(unsafe.Pointer(&item))) // IFileDialog::GetResult
		if failedHR(hr) || item == nil {
			return nil, hrErr(hr, "GetResult")
		}
		defer comRelease(item)
		p, err := shellItemPath(item)
		if err != nil || p == "" {
			return nil, err
		}
		return []string{p}, nil
	})
}
