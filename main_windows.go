//go:build windows

package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	comctl32 = syscall.NewLazyDLL("comctl32.dll")
	dwmapi   = syscall.NewLazyDLL("dwmapi.dll")
	msimg32  = syscall.NewLazyDLL("msimg32.dll")
	uxtheme  = syscall.NewLazyDLL("uxtheme.dll")

	pRegisterClassExW     = user32.NewProc("RegisterClassExW")
	pCreateWindowExW      = user32.NewProc("CreateWindowExW")
	pDefWindowProcW       = user32.NewProc("DefWindowProcW")
	pShowWindow           = user32.NewProc("ShowWindow")
	pUpdateWindow         = user32.NewProc("UpdateWindow")
	pGetMessageW          = user32.NewProc("GetMessageW")
	pTranslateMessage     = user32.NewProc("TranslateMessage")
	pDispatchMessageW     = user32.NewProc("DispatchMessageW")
	pPostQuitMessage      = user32.NewProc("PostQuitMessage")
	pPostMessageW         = user32.NewProc("PostMessageW")
	pSendMessageW         = user32.NewProc("SendMessageW")
	pSetWindowTextW       = user32.NewProc("SetWindowTextW")
	pGetWindowTextW       = user32.NewProc("GetWindowTextW")
	pGetWindowTextLengthW = user32.NewProc("GetWindowTextLengthW")
	pEnableWindow         = user32.NewProc("EnableWindow")
	pMessageBoxW          = user32.NewProc("MessageBoxW")
	pLoadCursorW          = user32.NewProc("LoadCursorW")
	pSetProcessDPIAware   = user32.NewProc("SetProcessDPIAware")
	pGetClientRect        = user32.NewProc("GetClientRect")
	pSetWindowPos         = user32.NewProc("SetWindowPos")
	pInvalidateRect       = user32.NewProc("InvalidateRect")
	pBeginPaint           = user32.NewProc("BeginPaint")
	pEndPaint             = user32.NewProc("EndPaint")
	pDrawTextW            = user32.NewProc("DrawTextW")
	pSetFocus             = user32.NewProc("SetFocus")

	pGetModuleHandleW = kernel32.NewProc("GetModuleHandleW")
	pDragAcceptFiles  = shell32.NewProc("DragAcceptFiles")
	pDragQueryFileW   = shell32.NewProc("DragQueryFileW")
	pDragFinish       = shell32.NewProc("DragFinish")
	pShellExecuteW    = shell32.NewProc("ShellExecuteW")

	pCreateFontW        = gdi32.NewProc("CreateFontW")
	pCreateSolidBrush   = gdi32.NewProc("CreateSolidBrush")
	pCreatePen          = gdi32.NewProc("CreatePen")
	pDeleteObject       = gdi32.NewProc("DeleteObject")
	pSelectObject       = gdi32.NewProc("SelectObject")
	pSetTextColor       = gdi32.NewProc("SetTextColor")
	pSetBkColor         = gdi32.NewProc("SetBkColor")
	pSetBkMode          = gdi32.NewProc("SetBkMode")
	pFillRect           = user32.NewProc("FillRect")
	pRoundRect          = gdi32.NewProc("RoundRect")
	pCreateRoundRectRgn = gdi32.NewProc("CreateRoundRectRgn")
	pSelectClipRgn      = gdi32.NewProc("SelectClipRgn")
	pGetStockObject     = gdi32.NewProc("GetStockObject")
	pMoveToEx           = gdi32.NewProc("MoveToEx")
	pLineTo             = gdi32.NewProc("LineTo")

	pInitCommonControlsEx  = comctl32.NewProc("InitCommonControlsEx")
	pDwmSetWindowAttribute = dwmapi.NewProc("DwmSetWindowAttribute")
	pGradientFill          = msimg32.NewProc("GradientFill")
	pSetWindowTheme        = uxtheme.NewProc("SetWindowTheme")
)

const (
	WS_OVERLAPPED   = 0x00000000
	WS_CAPTION      = 0x00C00000
	WS_SYSMENU      = 0x00080000
	WS_MINIMIZEBOX  = 0x00020000
	WS_MAXIMIZEBOX  = 0x00010000
	WS_THICKFRAME   = 0x00040000
	WS_VISIBLE      = 0x10000000
	WS_CHILD        = 0x40000000
	WS_BORDER       = 0x00800000
	WS_TABSTOP      = 0x00010000
	WS_VSCROLL      = 0x00200000
	WS_HSCROLL      = 0x00100000
	WS_CLIPCHILDREN = 0x02000000

	BS_OWNERDRAW         = 0x0000000B
	BS_AUTOCHECKBOX      = 0x00000003
	ES_AUTOHSCROLL       = 0x0080
	ES_READONLY          = 0x0800
	CBS_DROPDOWNLIST     = 0x0003
	LBS_NOTIFY           = 0x0001
	LBS_NOINTEGRALHEIGHT = 0x0100
	LBS_EXTENDEDSEL      = 0x0800

	LVS_REPORT        = 0x0001
	LVS_SHOWSELALWAYS = 0x0008

	CW_USEDEFAULT = 0x80000000
	SW_SHOW       = 5
	IDC_ARROW     = 32512

	WM_CREATE          = 0x0001
	WM_DESTROY         = 0x0002
	WM_SIZE            = 0x0005
	WM_PAINT           = 0x000F
	WM_ERASEBKGND      = 0x0014
	WM_COMMAND         = 0x0111
	WM_NOTIFY          = 0x004E
	WM_DRAWITEM        = 0x002B
	WM_CTLCOLORSTATIC  = 0x0138
	WM_CTLCOLOREDIT    = 0x0133
	WM_CTLCOLORLISTBOX = 0x0134
	WM_CTLCOLORBTN     = 0x0135
	WM_DROPFILES       = 0x0233
	WM_SETFONT         = 0x0030
	WM_GETMINMAXINFO   = 0x0024
	WM_APP             = 0x8000
	WM_USER            = 0x0400
	WM_ADD_FILES_DONE  = WM_APP + 1
	WM_ADD_FOLDER_DONE = WM_APP + 2
	WM_OUT_FOLDER_DONE = WM_APP + 3
	WM_BATCH_PROGRESS  = WM_APP + 4
	WM_BATCH_DONE      = WM_APP + 5

	BN_CLICKED    = 0
	CBN_SELCHANGE = 1
	CB_ADDSTRING  = 0x0143
	CB_SETCURSEL  = 0x014E
	CB_GETCURSEL  = 0x0147
	BM_GETCHECK   = 0x00F0
	BM_SETCHECK   = 0x00F1
	BST_CHECKED   = 1

	LB_ADDSTRING           = 0x0180
	LB_RESETCONTENT        = 0x0184
	LB_GETSELCOUNT         = 0x0190
	LB_GETSELITEMS         = 0x0191
	LB_SETHORIZONTALEXTENT = 0x0194

	LVM_FIRST                    = 0x1000
	LVM_SETBKCOLOR               = LVM_FIRST + 1
	LVM_DELETEALLITEMS           = LVM_FIRST + 9
	LVM_GETNEXTITEM              = LVM_FIRST + 12
	LVM_SETCOLUMNWIDTH           = LVM_FIRST + 30
	LVM_SETTEXTCOLOR             = LVM_FIRST + 36
	LVM_SETTEXTBKCOLOR           = LVM_FIRST + 38
	LVM_SETEXTENDEDLISTVIEWSTYLE = LVM_FIRST + 54
	LVM_INSERTITEMW              = LVM_FIRST + 77
	LVM_SETCOLUMNW               = LVM_FIRST + 96
	LVM_INSERTCOLUMNW            = LVM_FIRST + 97
	LVM_SETITEMTEXTW             = LVM_FIRST + 116

	LVS_EX_GRIDLINES     = 0x00000001
	LVS_EX_FULLROWSELECT = 0x00000020
	LVS_EX_DOUBLEBUFFER  = 0x00010000
	LVNI_SELECTED        = 0x0002
	LVIF_TEXT            = 0x0001
	LVCF_FMT             = 0x0001
	LVCF_WIDTH           = 0x0002
	LVCF_TEXT            = 0x0004
	LVCF_SUBITEM         = 0x0008
	LVCFMT_LEFT          = 0x0000
	LVCFMT_SORTDOWN      = 0x0200
	LVCFMT_SORTUP        = 0x0400
	LVN_COLUMNCLICK      = -108

	PBM_SETRANGE32  = WM_USER + 6
	PBM_SETPOS      = WM_USER + 2
	PBM_SETBARCOLOR = WM_USER + 9
	PBM_SETBKCOLOR  = 0x2001

	MB_OK        = 0x00000000
	MB_ICONERROR = 0x00000010

	ICC_LISTVIEW_CLASSES = 0x00000001
	ICC_PROGRESS_CLASS   = 0x00000020

	TRANSPARENT          = 1
	HOLLOW_BRUSH         = 5
	PS_SOLID             = 0
	DT_CENTER            = 0x0001
	DT_VCENTER           = 0x0004
	DT_SINGLELINE        = 0x0020
	ODS_SELECTED         = 0x0001
	ODS_DISABLED         = 0x0004
	ODS_FOCUS            = 0x0010
	SWP_NOZORDER         = 0x0004
	GRADIENT_FILL_RECT_V = 0x00000001
)

type WNDCLASSEX struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     syscall.Handle
	hIcon         syscall.Handle
	hCursor       syscall.Handle
	hbrBackground syscall.Handle
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       syscall.Handle
}

type POINT struct{ X, Y int32 }
type RECT struct{ Left, Top, Right, Bottom int32 }
type MSG struct {
	HWnd     syscall.Handle
	Message  uint32
	WParam   uintptr
	LParam   uintptr
	Time     uint32
	Pt       POINT
	LPrivate uint32
}
type PAINTSTRUCT struct {
	Hdc         syscall.Handle
	FErase      int32
	RcPaint     RECT
	FRestore    int32
	FIncUpdate  int32
	RgbReserved [32]byte
}
type DRAWITEMSTRUCT struct {
	CtlType    uint32
	CtlID      uint32
	ItemID     uint32
	ItemAction uint32
	ItemState  uint32
	HwndItem   syscall.Handle
	HDC        syscall.Handle
	RcItem     RECT
	ItemData   uintptr
}
type INITCOMMONCONTROLSEX struct {
	DwSize uint32
	DwICC  uint32
}
type MINMAXINFO struct {
	PtReserved     POINT
	PtMaxSize      POINT
	PtMaxPosition  POINT
	PtMinTrackSize POINT
	PtMaxTrackSize POINT
}
type TRIVERTEX struct {
	X, Y                    int32
	Red, Green, Blue, Alpha uint16
}
type GRADIENT_RECT struct{ UpperLeft, LowerRight uint32 }

type LVCOLUMNW struct {
	Mask       uint32
	Fmt        int32
	Cx         int32
	PszText    *uint16
	CchTextMax int32
	ISubItem   int32
	IImage     int32
	IOrder     int32
	CxMin      int32
	CxDefault  int32
	CxIdeal    int32
}

type LVITEMW struct {
	Mask       uint32
	IItem      int32
	ISubItem   int32
	State      uint32
	StateMask  uint32
	PszText    *uint16
	CchTextMax int32
	IImage     int32
	LParam     uintptr
	IIndent    int32
	IGroupID   int32
	CColumns   uint32
	PuColumns  *uint32
	PiColFmt   *int32
	IGroup     int32
}

type NMHDR struct {
	HwndFrom syscall.Handle
	IdFrom   uintptr
	Code     uint32
}

type NMLISTVIEW struct {
	Hdr       NMHDR
	IItem     int32
	ISubItem  int32
	UNewState uint32
	UOldState uint32
	UChanged  uint32
	PtAction  POINT
	LParam    uintptr
}

type AppSettings struct {
	OutputFolder    string `json:"output_folder"`
	View            int    `json:"view"`
	Resolution      int    `json:"resolution"`
	AA              int    `json:"aa"`
	Mirror          bool   `json:"mirror"`
	PreserveNormals bool   `json:"preserve_normals"`
	Overwrite       bool   `json:"overwrite"`
}

type BatchResult struct {
	Total, Rendered, Skipped, Failed int
	Cancelled                        bool
	Duration                         time.Duration
	Errors                           []string
}

type ButtonStyle struct {
	Top, Bottom, Border uint32
	Danger              bool
}

var (
	hwndMain                                                             syscall.Handle
	listModels, edOutput                                                 syscall.Handle
	cbView, cbRes, cbAA                                                  syscall.Handle
	ckMirror, ckOriginal, ckOverwrite                                    syscall.Handle
	btnAddFiles, btnAddFolder, btnRemove, btnClear                       syscall.Handle
	btnBrowseOutput, btnRender, btnCancel, btnOpenFolder                 syscall.Handle
	lblHeaderTitle, lblHeaderSub, lblHeaderBadge                         syscall.Handle
	lblCount, lblQueueTitle, lblQueueHint, lblOutputTitle, lblOutputHint syscall.Handle
	lblSettingsTitle, lblView, lblRes, lblAA, lblQuality, lblStatusTitle syscall.Handle
	status, progress                                                     syscall.Handle

	fontBody, fontTitle, fontSubtitle, fontSection, fontButton, fontSmall, fontBadge syscall.Handle
	brushPanel, brushEdit, brushList, brushTransparent                               syscall.Handle
	buttonStyles                                                                     = map[syscall.Handle]ButtonStyle{}
	staticColors                                                                     = map[syscall.Handle]uint32{}

	clientW, clientH                       int32
	queuePanel, settingsPanel, actionPanel RECT

	models                                []string
	settings                              AppSettings
	rendering, dialogBusy                 bool
	dialogMu                              sync.Mutex
	dialogPaths                           []string
	batchMu                               sync.Mutex
	batchProgressDone, batchProgressTotal int
	batchCurrent                          string
	batchResult                           BatchResult
	cancelMu                              sync.Mutex
	cancelRequested                       bool
	modelSortColumn                       int
	modelSortAscending                    bool
)

const (
	ID_LIST        = 101
	ID_ADD_FILES   = 102
	ID_ADD_FOLDER  = 103
	ID_REMOVE      = 104
	ID_CLEAR       = 105
	ID_OUTPUT      = 106
	ID_BROWSE_OUT  = 107
	ID_VIEW        = 108
	ID_RES         = 109
	ID_AA          = 110
	ID_MIRROR      = 111
	ID_ORIG        = 112
	ID_OVERWRITE   = 113
	ID_RENDER      = 114
	ID_CANCEL      = 115
	ID_OPEN_FOLDER = 116
)

func rgb(r, g, b uint32) uint32 { return r | (g << 8) | (b << 16) }
func w(s string) *uint16        { p, _ := syscall.UTF16PtrFromString(s); return p }
func text(h syscall.Handle) string {
	n, _, _ := pGetWindowTextLengthW.Call(uintptr(h))
	b := make([]uint16, int(n)+1)
	if len(b) == 0 {
		return ""
	}
	pGetWindowTextW.Call(uintptr(h), uintptr(unsafe.Pointer(&b[0])), n+1)
	return syscall.UTF16ToString(b)
}
func setText(h syscall.Handle, s string) {
	p := w(s)
	pSetWindowTextW.Call(uintptr(h), uintptr(unsafe.Pointer(p)))
	runtime.KeepAlive(p)
	// Runtime labels such as the status line and quality badge change text in place.
	// Force a full erase/repaint so transparent text from the previous value cannot
	// remain underneath the new value.
	if h != 0 && (h == status || h == lblQuality) {
		pInvalidateRect.Call(uintptr(h), 0, 1)
	}
}
func send(h syscall.Handle, msg uint32, wp, lp uintptr) uintptr {
	r, _, _ := pSendMessageW.Call(uintptr(h), uintptr(msg), wp, lp)
	return r
}
func loword(v uintptr) uint16 { return uint16(v & 0xffff) }
func hiword(v uintptr) uint16 { return uint16((v >> 16) & 0xffff) }

func makeFont(height int32, weight uint32) syscall.Handle {
	face := w("Segoe UI")
	f, _, _ := pCreateFontW.Call(uintptr(uint32(height)), 0, 0, 0, uintptr(weight), 0, 0, 0, 1, 0, 0, 5, 0, uintptr(unsafe.Pointer(face)))
	runtime.KeepAlive(face)
	return syscall.Handle(f)
}
func create(cls, txt string, style uint32, x, y, cx, cy int, id int) syscall.Handle {
	c, t := w(cls), w(txt)
	h, _, _ := pCreateWindowExW.Call(0, uintptr(unsafe.Pointer(c)), uintptr(unsafe.Pointer(t)), uintptr(style), uintptr(x), uintptr(y), uintptr(cx), uintptr(cy), uintptr(hwndMain), uintptr(id), 0, 0)
	runtime.KeepAlive(c)
	runtime.KeepAlive(t)
	hh := syscall.Handle(h)
	if fontBody != 0 && hh != 0 {
		send(hh, WM_SETFONT, uintptr(fontBody), 1)
	}
	return hh
}
func createLabel(txt string, font syscall.Handle, color uint32) syscall.Handle {
	h := create("STATIC", txt, WS_CHILD|WS_VISIBLE, 0, 0, 10, 10, 0)
	if font != 0 {
		send(h, WM_SETFONT, uintptr(font), 1)
	}
	staticColors[h] = color
	return h
}
func createButton(txt string, id int, top, bottom, border uint32) syscall.Handle {
	h := create("BUTTON", txt, WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_OWNERDRAW, 0, 0, 10, 10, id)
	send(h, WM_SETFONT, uintptr(fontButton), 1)
	buttonStyles[h] = ButtonStyle{Top: top, Bottom: bottom, Border: border}
	return h
}
func addCombo(h syscall.Handle, items []string, sel int) {
	for _, s := range items {
		p := w(s)
		send(h, CB_ADDSTRING, 0, uintptr(unsafe.Pointer(p)))
		runtime.KeepAlive(p)
	}
	if sel < 0 || sel >= len(items) {
		sel = 0
	}
	send(h, CB_SETCURSEL, uintptr(sel), 0)
}
func setCheck(h syscall.Handle, v bool) {
	if v {
		send(h, BM_SETCHECK, BST_CHECKED, 0)
	} else {
		send(h, BM_SETCHECK, 0, 0)
	}
}
func msgbox(s, title string, flags uintptr) {
	ss, tt := w(s), w(title)
	pMessageBoxW.Call(uintptr(hwndMain), uintptr(unsafe.Pointer(ss)), uintptr(unsafe.Pointer(tt)), flags)
	runtime.KeepAlive(ss)
	runtime.KeepAlive(tt)
}
func move(h syscall.Handle, x, y, cx, cy int32) {
	if h != 0 {
		pSetWindowPos.Call(uintptr(h), 0, uintptr(x), uintptr(y), uintptr(cx), uintptr(cy), SWP_NOZORDER)
	}
}

func settingsPath() string {
	base, err := os.UserConfigDir()
	if err != nil || base == "" {
		base = os.TempDir()
	}
	return filepath.Join(base, "OBJ2PNG Studio Pro", "settings.json")
}
func oldSettingsPath() string {
	base, err := os.UserConfigDir()
	if err != nil || base == "" {
		base = os.TempDir()
	}
	return filepath.Join(base, "OBJ2PNG Studio Pro", "settings.json")
}
func defaultOutputFolder() string {
	home, err := os.UserHomeDir()
	if err == nil && home != "" {
		return filepath.Join(home, "Pictures", "OBJ2PNG Renders")
	}
	return filepath.Join(os.TempDir(), "OBJ2PNG Renders")
}
func loadSettings() AppSettings {
	// v6 launches in max-quality mode by default. If an older install exists, only migrate the output folder.
	s := AppSettings{OutputFolder: defaultOutputFolder(), View: 0, Resolution: 2, AA: 1, Mirror: true, PreserveNormals: true, Overwrite: true}
	if b, err := os.ReadFile(settingsPath()); err == nil {
		_ = json.Unmarshal(b, &s)
	} else if b, err := os.ReadFile(oldSettingsPath()); err == nil {
		var old AppSettings
		if json.Unmarshal(b, &old) == nil && strings.TrimSpace(old.OutputFolder) != "" {
			s.OutputFolder = old.OutputFolder
		}
	}
	if strings.TrimSpace(s.OutputFolder) == "" {
		s.OutputFolder = defaultOutputFolder()
	}
	if s.View < 0 || s.View > 3 {
		s.View = 0
	}
	if s.Resolution < 0 || s.Resolution > 2 {
		s.Resolution = 2
	}
	if s.AA < 0 || s.AA > 1 {
		s.AA = 1
	}
	return s
}
func readSettingsFromUI() AppSettings {
	s := settings
	s.OutputFolder = strings.TrimSpace(text(edOutput))
	s.View = int(send(cbView, CB_GETCURSEL, 0, 0))
	s.Resolution = int(send(cbRes, CB_GETCURSEL, 0, 0))
	s.AA = int(send(cbAA, CB_GETCURSEL, 0, 0))
	s.Mirror = send(ckMirror, BM_GETCHECK, 0, 0) == BST_CHECKED
	s.PreserveNormals = send(ckOriginal, BM_GETCHECK, 0, 0) == BST_CHECKED
	s.Overwrite = send(ckOverwrite, BM_GETCHECK, 0, 0) == BST_CHECKED
	return s
}
func saveSettings() {
	if hwndMain != 0 {
		settings = readSettingsFromUI()
	}
	p := settingsPath()
	_ = os.MkdirAll(filepath.Dir(p), 0755)
	b, _ := json.MarshalIndent(settings, "", "  ")
	_ = os.WriteFile(p, b, 0644)
}

func isMaxQualityPreset() bool {
	// The quality badge describes image quality only. Camera/view choices such as
	// Auto Side, Left/Right, and Mirror do not reduce render quality, and
	// Preserve OBJ normals is a geometry/fidelity option rather than a quality tier.
	// Therefore only the maximum resolution and anti-aliasing settings control
	// whether the badge says MAX QUALITY or CUSTOM QUALITY.
	if cbRes == 0 || cbAA == 0 {
		return true
	}
	return int(send(cbRes, CB_GETCURSEL, 0, 0)) == 2 &&
		int(send(cbAA, CB_GETCURSEL, 0, 0)) == 1
}

func qualityLabelText() string {
	if isMaxQualityPreset() {
		return "MAX QUALITY"
	}
	return "CUSTOM QUALITY"
}

func updateQualityBadge() {
	if lblQuality == 0 {
		return
	}
	if isMaxQualityPreset() {
		setText(lblQuality, "MAX QUALITY")
		staticColors[lblQuality] = rgb(119, 255, 190)
	} else {
		setText(lblQuality, "CUSTOM QUALITY")
		staticColors[lblQuality] = rgb(255, 189, 92)
	}
	pInvalidateRect.Call(uintptr(lblQuality), 0, 1)
}

func modelKey(p string) string {
	a, err := filepath.Abs(p)
	if err == nil {
		p = a
	}
	return strings.ToLower(filepath.Clean(p))
}
func addModelFile(p string, seen map[string]bool) int {
	if !strings.EqualFold(filepath.Ext(p), ".obj") {
		return 0
	}
	a, err := filepath.Abs(p)
	if err == nil {
		p = a
	}
	k := modelKey(p)
	if seen[k] {
		return 0
	}
	if st, err := os.Stat(p); err != nil || st.IsDir() {
		return 0
	}
	models = append(models, p)
	seen[k] = true
	return 1
}
func addPath(p string) int {
	seen := make(map[string]bool, len(models)+16)
	for _, m := range models {
		seen[modelKey(m)] = true
	}
	st, err := os.Stat(p)
	if err != nil {
		return 0
	}
	if !st.IsDir() {
		return addModelFile(p, seen)
	}
	added := 0
	_ = filepath.WalkDir(p, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if strings.EqualFold(filepath.Ext(path), ".obj") {
			added += addModelFile(path, seen)
		}
		return nil
	})
	return added
}
func addPaths(paths []string) int {
	added := 0
	for _, p := range paths {
		added += addPath(p)
	}
	if added > 0 {
		sortModelTable(modelSortColumn, false)
		refreshModelList()
	}
	return added
}
func initModelTable() {
	// Native Details/Report view: File name | Location.
	for i, col := range []struct {
		name  string
		width int32
	}{
		{"File name", 260},
		{"Location", 650},
	} {
		t := w(col.name)
		c := LVCOLUMNW{
			Mask: LVCF_FMT | LVCF_WIDTH | LVCF_TEXT | LVCF_SUBITEM,
			Fmt:  LVCFMT_LEFT, Cx: col.width, PszText: t, ISubItem: int32(i),
		}
		send(listModels, LVM_INSERTCOLUMNW, uintptr(i), uintptr(unsafe.Pointer(&c)))
		runtime.KeepAlive(t)
	}
	ex := uintptr(LVS_EX_FULLROWSELECT | LVS_EX_GRIDLINES | LVS_EX_DOUBLEBUFFER)
	send(listModels, LVM_SETEXTENDEDLISTVIEWSTYLE, ex, ex)
	send(listModels, LVM_SETBKCOLOR, 0, uintptr(rgb(25, 33, 56)))
	send(listModels, LVM_SETTEXTBKCOLOR, 0, uintptr(rgb(25, 33, 56)))
	send(listModels, LVM_SETTEXTCOLOR, 0, uintptr(rgb(222, 232, 248)))
	updateModelSortIndicator()
}

func updateModelSortIndicator() {
	for col := 0; col < 2; col++ {
		fmtVal := int32(LVCFMT_LEFT)
		if col == modelSortColumn {
			if modelSortAscending {
				fmtVal |= LVCFMT_SORTUP
			} else {
				fmtVal |= LVCFMT_SORTDOWN
			}
		}
		c := LVCOLUMNW{Mask: LVCF_FMT, Fmt: fmtVal}
		send(listModels, LVM_SETCOLUMNW, uintptr(col), uintptr(unsafe.Pointer(&c)))
	}
}

func sortModelTable(column int, toggle bool) {
	if toggle && modelSortColumn == column {
		modelSortAscending = !modelSortAscending
	} else {
		modelSortColumn = column
		modelSortAscending = true
	}
	sort.SliceStable(models, func(i, j int) bool {
		var a, b string
		if modelSortColumn == 1 {
			a, b = filepath.Dir(models[i]), filepath.Dir(models[j])
		} else {
			a, b = filepath.Base(models[i]), filepath.Base(models[j])
		}
		a, b = strings.ToLower(a), strings.ToLower(b)
		if a == b {
			a, b = strings.ToLower(models[i]), strings.ToLower(models[j])
		}
		if modelSortAscending {
			return a < b
		}
		return a > b
	})
	updateModelSortIndicator()
}

func refreshModelList() {
	send(listModels, LVM_DELETEALLITEMS, 0, 0)
	for i, p := range models {
		name := w(filepath.Base(p))
		item := LVITEMW{Mask: LVIF_TEXT, IItem: int32(i), ISubItem: 0, PszText: name}
		send(listModels, LVM_INSERTITEMW, 0, uintptr(unsafe.Pointer(&item)))
		runtime.KeepAlive(name)

		loc := w(filepath.Dir(p))
		sub := LVITEMW{IItem: int32(i), ISubItem: 1, PszText: loc}
		send(listModels, LVM_SETITEMTEXTW, uintptr(i), uintptr(unsafe.Pointer(&sub)))
		runtime.KeepAlive(loc)
	}
	word := "MODELS"
	if len(models) == 1 {
		word = "MODEL"
	}
	setText(lblCount, fmt.Sprintf("%d %s QUEUED", len(models), word))
	pInvalidateRect.Call(uintptr(hwndMain), 0, 0)
}

func selectedModelRows() []int {
	var out []int
	idx := int32(-1)
	for {
		r := send(listModels, LVM_GETNEXTITEM, uintptr(int64(idx)), LVNI_SELECTED)
		next := int32(uint32(r))
		if next < 0 {
			break
		}
		out = append(out, int(next))
		idx = next
	}
	return out
}

func removeSelectedModels() {
	rows := selectedModelRows()
	if len(rows) == 0 {
		return
	}
	rm := map[int]bool{}
	for _, i := range rows {
		rm[i] = true
	}
	kept := models[:0]
	for i, p := range models {
		if !rm[i] {
			kept = append(kept, p)
		}
	}
	models = kept
	refreshModelList()
}

func startDialog(message uint32, work func() ([]string, error), busyText string) {
	if dialogBusy || rendering {
		return
	}
	dialogBusy = true
	enableInputControls(false)
	setText(status, busyText)
	go func() {
		paths, err := work()
		dialogMu.Lock()
		if err != nil {
			dialogPaths = []string{"__ERROR__", err.Error()}
		} else {
			dialogPaths = paths
		}
		dialogMu.Unlock()
		pPostMessageW.Call(uintptr(hwndMain), uintptr(message), 0, 0)
	}()
}
func takeDialogResult() ([]string, error) {
	dialogMu.Lock()
	p := dialogPaths
	dialogPaths = nil
	dialogMu.Unlock()
	dialogBusy = false
	enableInputControls(true)
	if len(p) >= 2 && p[0] == "__ERROR__" {
		return nil, fmt.Errorf("%s", p[1])
	}
	return p, nil
}
func startAddFiles() {
	startDialog(WM_ADD_FILES_DONE, func() ([]string, error) { return modernPickOBJFiles(hwndMain) }, "Opening Windows file picker...")
}
func startAddFolder() {
	startDialog(WM_ADD_FOLDER_DONE, func() ([]string, error) { return modernPickFolder(hwndMain, "Add a folder containing OBJ models", "") }, "Opening Windows folder picker...")
}
func startOutputFolder() {
	initial := strings.TrimSpace(text(edOutput))
	startDialog(WM_OUT_FOLDER_DONE, func() ([]string, error) {
		return modernPickFolder(hwndMain, "Choose the default render output folder", initial)
	}, "Choosing output folder...")
}
func finishAddFiles() {
	p, err := takeDialogResult()
	if err != nil {
		msgbox(err.Error(), "Add models", MB_OK|MB_ICONERROR)
		setText(status, "Could not open the Windows file picker.")
		return
	}
	n := addPaths(p)
	if n > 0 {
		setText(status, fmt.Sprintf("Added %d OBJ model(s). Ready to render.", n))
	} else {
		setText(status, "No new OBJ models added.")
	}
}
func finishAddFolder() {
	p, err := takeDialogResult()
	if err != nil {
		msgbox(err.Error(), "Add folder", MB_OK|MB_ICONERROR)
		setText(status, "Could not open the Windows folder picker.")
		return
	}
	n := addPaths(p)
	if n > 0 {
		setText(status, fmt.Sprintf("Added %d OBJ model(s) from folder.", n))
	} else {
		setText(status, "No OBJ models found in that folder.")
	}
}
func finishOutputFolder() {
	p, err := takeDialogResult()
	if err != nil {
		msgbox(err.Error(), "Output folder", MB_OK|MB_ICONERROR)
		setText(status, "Could not open the Windows folder picker.")
		return
	}
	if len(p) > 0 && p[0] != "" {
		setText(edOutput, p[0])
		settings.OutputFolder = p[0]
		saveSettings()
		setText(status, "Output folder saved. Future renders go there automatically.")
	} else {
		setText(status, "Output folder selection cancelled.")
	}
}

func enableInputControls(on bool) {
	v := uintptr(0)
	if on {
		v = 1
	}
	for _, h := range []syscall.Handle{btnAddFiles, btnAddFolder, btnRemove, btnClear, btnBrowseOutput, cbView, cbRes, cbAA, ckMirror, ckOriginal, ckOverwrite, btnRender} {
		if h != 0 {
			pEnableWindow.Call(uintptr(h), v)
		}
	}
	if btnCancel != 0 {
		if rendering {
			pEnableWindow.Call(uintptr(btnCancel), 1)
		} else {
			pEnableWindow.Call(uintptr(btnCancel), 0)
		}
	}
	pInvalidateRect.Call(uintptr(hwndMain), 0, 0)
}
func collectRenderOptions() (RenderOptions, string, bool, error) {
	if len(models) == 0 {
		return RenderOptions{}, "", false, fmt.Errorf("add or drop at least one OBJ model first")
	}
	outDir := strings.TrimSpace(text(edOutput))
	if outDir == "" {
		return RenderOptions{}, "", false, fmt.Errorf("choose an output folder first")
	}
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return RenderOptions{}, "", false, fmt.Errorf("cannot create output folder: %v", err)
	}
	st, err := os.Stat(outDir)
	if err != nil || !st.IsDir() {
		return RenderOptions{}, "", false, fmt.Errorf("output path is not a folder")
	}
	o := DefaultRenderOptions()
	switch int(send(cbRes, CB_GETCURSEL, 0, 0)) {
	case 0:
		o.Width, o.Height = 2000, 1000
	case 1:
		o.Width, o.Height = 3000, 1500
	default:
		o.Width, o.Height = 4000, 2000
	}
	views := []string{"auto", "x", "y", "z"}
	vi := int(send(cbView, CB_GETCURSEL, 0, 0))
	if vi >= 0 && vi < len(views) {
		o.View = views[vi]
	}
	if int(send(cbAA, CB_GETCURSEL, 0, 0)) == 1 {
		o.AA = 2
	} else {
		o.AA = 1
	}
	o.Mirror = send(ckMirror, BM_GETCHECK, 0, 0) == BST_CHECKED
	o.UseOriginalNormals = send(ckOriginal, BM_GETCHECK, 0, 0) == BST_CHECKED
	overwrite := send(ckOverwrite, BM_GETCHECK, 0, 0) == BST_CHECKED
	return o, outDir, overwrite, nil
}
func makeOutputPaths(inputs []string, outDir string) []string {
	result := make([]string, len(inputs))
	used := map[string]bool{}
	for i, in := range inputs {
		base := strings.TrimSuffix(filepath.Base(in), filepath.Ext(in))
		candidate := filepath.Join(outDir, base+".png")
		key := strings.ToLower(candidate)
		n := 2
		for used[key] {
			candidate = filepath.Join(outDir, base+"_"+strconv.Itoa(n)+".png")
			key = strings.ToLower(candidate)
			n++
		}
		used[key] = true
		result[i] = candidate
	}
	return result
}
func setCancel(v bool)        { cancelMu.Lock(); cancelRequested = v; cancelMu.Unlock() }
func isCancelRequested() bool { cancelMu.Lock(); v := cancelRequested; cancelMu.Unlock(); return v }
func postBatchProgress(done, total int, current string) {
	batchMu.Lock()
	batchProgressDone = done
	batchProgressTotal = total
	batchCurrent = current
	batchMu.Unlock()
	pPostMessageW.Call(uintptr(hwndMain), WM_BATCH_PROGRESS, 0, 0)
}
func startBatchRender() {
	if rendering || dialogBusy {
		return
	}
	opt, outDir, overwrite, err := collectRenderOptions()
	if err != nil {
		msgbox(err.Error(), "OBJ2PNG Studio", MB_OK|MB_ICONERROR)
		return
	}
	settings = readSettingsFromUI()
	saveSettings()
	inputs := append([]string(nil), models...)
	outputs := makeOutputPaths(inputs, outDir)
	rendering = true
	setCancel(false)
	enableInputControls(false)
	send(progress, PBM_SETRANGE32, 0, uintptr(len(inputs)))
	send(progress, PBM_SETPOS, 0, 0)
	setText(status, fmt.Sprintf("%s batch started - %d model(s).", qualityLabelText(), len(inputs)))
	go func() {
		started := time.Now()
		r := BatchResult{Total: len(inputs)}
		for i, in := range inputs {
			if isCancelRequested() {
				r.Cancelled = true
				break
			}
			postBatchProgress(i, len(inputs), filepath.Base(in))
			out := outputs[i]
			if !overwrite {
				if _, err := os.Stat(out); err == nil {
					r.Skipped++
					continue
				}
			}
			if err := RenderOBJ(in, out, opt); err != nil {
				r.Failed++
				r.Errors = append(r.Errors, filepath.Base(in)+": "+err.Error())
			} else {
				r.Rendered++
			}
		}
		r.Duration = time.Since(started)
		batchMu.Lock()
		batchResult = r
		batchMu.Unlock()
		pPostMessageW.Call(uintptr(hwndMain), WM_BATCH_DONE, 0, 0)
	}()
}
func updateBatchProgress() {
	batchMu.Lock()
	done, total, current := batchProgressDone, batchProgressTotal, batchCurrent
	batchMu.Unlock()
	send(progress, PBM_SETRANGE32, 0, uintptr(total))
	send(progress, PBM_SETPOS, uintptr(done), 0)
	if total > 0 {
		setText(status, fmt.Sprintf("Rendering %d/%d  -  %s", done+1, total, current))
	}
}
func finishBatchRender() {
	batchMu.Lock()
	r := batchResult
	batchMu.Unlock()
	rendering = false
	setCancel(false)
	enableInputControls(true)
	send(progress, PBM_SETRANGE32, 0, uintptr(r.Total))
	if !r.Cancelled {
		send(progress, PBM_SETPOS, uintptr(r.Total), 0)
	}
	outDir := strings.TrimSpace(text(edOutput))
	if len(r.Errors) > 0 {
		_ = os.WriteFile(filepath.Join(outDir, "OBJ2PNG_errors.txt"), []byte(strings.Join(r.Errors, "\r\n")+"\r\n"), 0644)
	}
	if r.Cancelled {
		done := r.Rendered + r.Skipped + r.Failed
		setText(status, fmt.Sprintf("Cancelled after %d/%d. Rendered %d, skipped %d, failed %d.", done, r.Total, r.Rendered, r.Skipped, r.Failed))
		return
	}
	setText(status, fmt.Sprintf("Complete - %d rendered, %d skipped, %d failed in %s.", r.Rendered, r.Skipped, r.Failed, r.Duration.Round(time.Second)))
	if r.Failed > 0 {
		msgbox(fmt.Sprintf("%d model(s) failed. See OBJ2PNG_errors.txt in the output folder.", r.Failed), "Batch completed with errors", MB_OK|MB_ICONERROR)
	}
	// A completed batch should take the user straight to the rendered files.
	openOutputFolder()
}
func cancelBatch() {
	if !rendering {
		return
	}
	setCancel(true)
	pEnableWindow.Call(uintptr(btnCancel), 0)
	setText(status, "Cancel requested - finishing the current model first...")
}
func openOutputFolder() {
	p := strings.TrimSpace(text(edOutput))
	if p == "" {
		return
	}
	_ = os.MkdirAll(p, 0755)
	verb, target := w("open"), w(p)
	pShellExecuteW.Call(uintptr(hwndMain), uintptr(unsafe.Pointer(verb)), uintptr(unsafe.Pointer(target)), 0, 0, 1)
	runtime.KeepAlive(verb)
	runtime.KeepAlive(target)
}
func handleDrop(hdrop syscall.Handle) {
	if rendering {
		pDragFinish.Call(uintptr(hdrop))
		return
	}
	count, _, _ := pDragQueryFileW.Call(uintptr(hdrop), 0xffffffff, 0, 0)
	paths := make([]string, 0, int(count))
	for i := uintptr(0); i < count; i++ {
		n, _, _ := pDragQueryFileW.Call(uintptr(hdrop), i, 0, 0)
		if n == 0 {
			continue
		}
		buf := make([]uint16, int(n)+1)
		pDragQueryFileW.Call(uintptr(hdrop), i, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
		if p := syscall.UTF16ToString(buf); p != "" {
			paths = append(paths, p)
		}
	}
	pDragFinish.Call(uintptr(hdrop))
	n := addPaths(paths)
	if n > 0 {
		setText(status, fmt.Sprintf("Added %d model(s) by drag and drop.", n))
	} else {
		setText(status, "No new OBJ models found in the dropped items.")
	}
}

func gradientFillRect(hdc syscall.Handle, r RECT, top, bottom uint32) {
	verts := []TRIVERTEX{{r.Left, r.Top, uint16((top & 0xff) << 8), uint16(((top >> 8) & 0xff) << 8), uint16(((top >> 16) & 0xff) << 8), 0xffff}, {r.Right, r.Bottom, uint16((bottom & 0xff) << 8), uint16(((bottom >> 8) & 0xff) << 8), uint16(((bottom >> 16) & 0xff) << 8), 0xffff}}
	gr := GRADIENT_RECT{0, 1}
	pGradientFill.Call(uintptr(hdc), uintptr(unsafe.Pointer(&verts[0])), 2, uintptr(unsafe.Pointer(&gr)), 1, GRADIENT_FILL_RECT_V)
}
func fillRound(hdc syscall.Handle, r RECT, color uint32, radius int32) {
	br, _, _ := pCreateSolidBrush.Call(uintptr(color))
	old, _, _ := pSelectObject.Call(uintptr(hdc), br)
	pen, _, _ := pCreatePen.Call(PS_SOLID, 1, uintptr(color))
	oldp, _, _ := pSelectObject.Call(uintptr(hdc), pen)
	pRoundRect.Call(uintptr(hdc), uintptr(r.Left), uintptr(r.Top), uintptr(r.Right), uintptr(r.Bottom), uintptr(radius), uintptr(radius))
	pSelectObject.Call(uintptr(hdc), old)
	pSelectObject.Call(uintptr(hdc), oldp)
	pDeleteObject.Call(br)
	pDeleteObject.Call(pen)
}
func drawPanel(hdc syscall.Handle, r RECT) {
	shadow := RECT{r.Left + 4, r.Top + 5, r.Right + 4, r.Bottom + 5}
	fillRound(hdc, shadow, rgb(5, 8, 20), 18)
	fillRound(hdc, r, rgb(20, 27, 48), 18)
	pen, _, _ := pCreatePen.Call(PS_SOLID, 1, uintptr(rgb(55, 70, 106)))
	old, _, _ := pSelectObject.Call(uintptr(hdc), pen)
	hollow, _, _ := pGetStockObject.Call(HOLLOW_BRUSH)
	oldb, _, _ := pSelectObject.Call(uintptr(hdc), hollow)
	pRoundRect.Call(uintptr(hdc), uintptr(r.Left), uintptr(r.Top), uintptr(r.Right), uintptr(r.Bottom), 18, 18)
	pSelectObject.Call(uintptr(hdc), old)
	pSelectObject.Call(uintptr(hdc), oldb)
	pDeleteObject.Call(pen)
}
func drawButton(dis *DRAWITEMSTRUCT) {
	st, ok := buttonStyles[dis.HwndItem]
	if !ok {
		return
	}
	r := dis.RcItem
	pressed := dis.ItemState&ODS_SELECTED != 0
	disabled := dis.ItemState&ODS_DISABLED != 0
	shadow := RECT{r.Left + 2, r.Top + 4, r.Right - 1, r.Bottom}
	fillRound(dis.HDC, shadow, rgb(5, 8, 18), 12)
	top, bottom, border := st.Top, st.Bottom, st.Border
	if disabled {
		top, bottom, border = rgb(67, 73, 91), rgb(45, 50, 66), rgb(82, 89, 110)
	}
	if pressed {
		top, bottom = bottom, top
		r.Top += 2
		r.Bottom += 2
	}
	rgn, _, _ := pCreateRoundRectRgn.Call(uintptr(r.Left), uintptr(r.Top), uintptr(r.Right), uintptr(r.Bottom), 12, 12)
	pSelectClipRgn.Call(uintptr(dis.HDC), rgn)
	gradientFillRect(dis.HDC, r, top, bottom)
	pSelectClipRgn.Call(uintptr(dis.HDC), 0)
	pDeleteObject.Call(rgn)
	pen, _, _ := pCreatePen.Call(PS_SOLID, 1, uintptr(border))
	oldp, _, _ := pSelectObject.Call(uintptr(dis.HDC), pen)
	hollow, _, _ := pGetStockObject.Call(HOLLOW_BRUSH)
	oldb, _, _ := pSelectObject.Call(uintptr(dis.HDC), hollow)
	pRoundRect.Call(uintptr(dis.HDC), uintptr(r.Left), uintptr(r.Top), uintptr(r.Right), uintptr(r.Bottom), 12, 12)
	pSelectObject.Call(uintptr(dis.HDC), oldp)
	pSelectObject.Call(uintptr(dis.HDC), oldb)
	pDeleteObject.Call(pen)
	pSetBkMode.Call(uintptr(dis.HDC), TRANSPARENT)
	tc := rgb(245, 249, 255)
	if disabled {
		tc = rgb(155, 163, 183)
	}
	pSetTextColor.Call(uintptr(dis.HDC), uintptr(tc))
	oldf, _, _ := pSelectObject.Call(uintptr(dis.HDC), uintptr(fontButton))
	s := text(dis.HwndItem)
	sp := w(s)
	pDrawTextW.Call(uintptr(dis.HDC), uintptr(unsafe.Pointer(sp)), uintptr(len(syscall.StringToUTF16(s))-1), uintptr(unsafe.Pointer(&r)), DT_CENTER|DT_VCENTER|DT_SINGLELINE)
	runtime.KeepAlive(sp)
	pSelectObject.Call(uintptr(dis.HDC), oldf)
}

func layoutControls(cw, ch int32) {
	if cw < 1120 {
		cw = 1120
	}
	if ch < 760 {
		ch = 760
	}
	clientW, clientH = cw, ch

	margin := int32(26)
	gap := int32(20)
	headerH := int32(94)
	bottomH := int32(146)
	contentTop := headerH + 18
	contentBottom := ch - bottomH - 14

	// Give Render Setup enough horizontal room so labels and controls never collide.
	available := cw - 2*margin - gap
	leftW := int32(float64(available) * 0.57)
	if leftW < 590 {
		leftW = 590
	}
	rightW := available - leftW
	if rightW < 455 {
		rightW = 455
		leftW = available - rightW
	}
	queuePanel = RECT{margin, contentTop, margin + leftW, contentBottom}
	settingsPanel = RECT{queuePanel.Right + gap, contentTop, cw - margin, contentBottom}
	actionPanel = RECT{margin, ch - bottomH, cw - margin, ch - 18}

	// Header: clear separation between title, subtitle and status badge.
	move(lblHeaderTitle, 28, 16, 500, 42)
	move(lblHeaderSub, 30, 58, cw-60, 24)

	// Model Queue card.
	move(lblQueueTitle, queuePanel.Left+22, queuePanel.Top+18, 260, 28)
	move(lblCount, queuePanel.Right-194, queuePanel.Top+20, 170, 24)
	move(lblQueueHint, queuePanel.Left+22, queuePanel.Top+49, queuePanel.Right-queuePanel.Left-44, 22)
	listY := queuePanel.Top + 82
	buttonsH := int32(48)
	listBottom := queuePanel.Bottom - 82
	tableW := queuePanel.Right - queuePanel.Left - 44
	move(listModels, queuePanel.Left+22, listY, tableW, listBottom-listY)
	// Keep File name compact and let Location consume the remaining width.
	fileColW := int32(float64(tableW) * 0.31)
	if fileColW < 220 {
		fileColW = 220
	}
	if fileColW > 330 {
		fileColW = 330
	}
	locColW := tableW - fileColW - 5
	if locColW < 240 {
		locColW = 240
	}
	send(listModels, LVM_SETCOLUMNWIDTH, 0, uintptr(fileColW))
	send(listModels, LVM_SETCOLUMNWIDTH, 1, uintptr(locColW))
	by := queuePanel.Bottom - 62
	qAvail := queuePanel.Right - queuePanel.Left - 44
	bw := (qAvail - 3*10) / 4
	move(btnAddFiles, queuePanel.Left+22, by, bw, buttonsH)
	move(btnAddFolder, queuePanel.Left+22+bw+10, by, bw, buttonsH)
	move(btnRemove, queuePanel.Left+22+2*(bw+10), by, bw, buttonsH)
	move(btnClear, queuePanel.Left+22+3*(bw+10), by, bw, buttonsH)

	// Render Setup card. Controls use vertical rhythm instead of squeezing 3 columns together.
	x := settingsPanel.Left + 24
	wcol := settingsPanel.Right - settingsPanel.Left - 48
	move(lblSettingsTitle, x, settingsPanel.Top+18, wcol-214, 30)
	move(lblQuality, settingsPanel.Right-202, settingsPanel.Top+20, 176, 24)

	y := settingsPanel.Top + 66
	move(lblOutputTitle, x, y, wcol, 22)
	y += 28
	editW := wcol - 136
	move(edOutput, x, y, editW, 38)
	move(btnBrowseOutput, x+editW+12, y, 124, 38)
	y += 46
	move(lblOutputHint, x, y, wcol, 20)

	// View + Resolution row.
	y += 42
	colGap := int32(14)
	viewW := int32(float64(wcol-colGap) * 0.42)
	resW := wcol - colGap - viewW
	move(lblView, x, y, viewW, 20)
	move(lblRes, x+viewW+colGap, y, resW, 20)
	y += 23
	move(cbView, x, y, viewW, 190)
	move(cbRes, x+viewW+colGap, y, resW, 190)

	// Anti-aliasing gets its own full-width row so the selected text is never clipped.
	y += 54
	move(lblAA, x, y, wcol, 20)
	y += 23
	move(cbAA, x, y, wcol, 170)

	// Independent option rows with generous spacing.
	y += 54
	move(ckMirror, x, y, wcol, 25)
	y += 34
	move(ckOriginal, x, y, wcol, 25)
	y += 34
	move(ckOverwrite, x, y, wcol, 25)

	// Bottom action bar.
	move(lblStatusTitle, actionPanel.Left+20, actionPanel.Top+13, 92, 20)
	move(status, actionPanel.Left+110, actionPanel.Top+12, actionPanel.Right-actionPanel.Left-130, 24)
	move(progress, actionPanel.Left+20, actionPanel.Top+42, actionPanel.Right-actionPanel.Left-40, 14)
	ay := actionPanel.Top + 68
	aw := actionPanel.Right - actionPanel.Left - 40
	renderW := int32(float64(aw) * 0.50)
	cancelW := int32(126)
	openW := aw - renderW - cancelW - 20
	move(btnRender, actionPanel.Left+20, ay, renderW, 42)
	move(btnCancel, actionPanel.Left+20+renderW+10, ay, cancelW, 42)
	move(btnOpenFolder, actionPanel.Left+20+renderW+cancelW+20, ay, openW, 42)

	pInvalidateRect.Call(uintptr(hwndMain), 0, 1)
}
func paintWindow(hwnd syscall.Handle) {
	var ps PAINTSTRUCT
	hdc, _, _ := pBeginPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&ps)))
	if hdc == 0 {
		return
	}
	defer pEndPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&ps)))
	r := RECT{0, 0, clientW, clientH}
	gradientFillRect(syscall.Handle(hdc), r, rgb(9, 14, 29), rgb(18, 13, 38))
	header := RECT{0, 0, clientW, 92}
	gradientFillRect(syscall.Handle(hdc), header, rgb(19, 48, 92), rgb(65, 25, 103))
	drawPanel(syscall.Handle(hdc), queuePanel)
	drawPanel(syscall.Handle(hdc), settingsPanel)
	drawPanel(syscall.Handle(hdc), actionPanel) // accent streak
	pen, _, _ := pCreatePen.Call(PS_SOLID, 3, uintptr(rgb(61, 210, 255)))
	old, _, _ := pSelectObject.Call(hdc, pen)
	pMoveToEx.Call(hdc, 24, 86, 0)
	pLineTo.Call(hdc, uintptr(clientW-24), 86)
	pSelectObject.Call(hdc, old)
	pDeleteObject.Call(pen)
}

func applyDarkTitleBar(hwnd syscall.Handle) {
	v := int32(1)
	pDwmSetWindowAttribute.Call(uintptr(hwnd), 20, uintptr(unsafe.Pointer(&v)), unsafe.Sizeof(v))
	corner := int32(2)
	pDwmSetWindowAttribute.Call(uintptr(hwnd), 33, uintptr(unsafe.Pointer(&corner)), unsafe.Sizeof(corner))
}
func applyExplorerTheme(h syscall.Handle) {
	// Windows 10/11 dark common-control theme when available; fall back safely.
	dark := w("DarkMode_Explorer")
	r, _, _ := pSetWindowTheme.Call(uintptr(h), uintptr(unsafe.Pointer(dark)), 0)
	runtime.KeepAlive(dark)
	if r != 0 {
		theme := w("Explorer")
		pSetWindowTheme.Call(uintptr(h), uintptr(unsafe.Pointer(theme)), 0)
		runtime.KeepAlive(theme)
	}
}

func wndProc(hwnd syscall.Handle, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case WM_CREATE:
		hwndMain = hwnd
		fontBody = makeFont(-18, 400)
		fontTitle = makeFont(-34, 700)
		fontSubtitle = makeFont(-18, 400)
		fontSection = makeFont(-22, 650)
		fontButton = makeFont(-17, 650)
		fontSmall = makeFont(-15, 400)
		fontBadge = makeFont(-15, 700)
		b, _, _ := pCreateSolidBrush.Call(uintptr(rgb(20, 27, 48)))
		brushPanel = syscall.Handle(b)
		b, _, _ = pCreateSolidBrush.Call(uintptr(rgb(31, 39, 63)))
		brushEdit = syscall.Handle(b)
		b, _, _ = pCreateSolidBrush.Call(uintptr(rgb(25, 33, 56)))
		brushList = syscall.Handle(b)
		hb, _, _ := pGetStockObject.Call(HOLLOW_BRUSH)
		brushTransparent = syscall.Handle(hb)
		// Header labels
		lblHeaderTitle = createLabel("OBJ2PNG STUDIO", fontTitle, rgb(246, 249, 255))
		lblHeaderSub = createLabel("OBJ to PNG Converter", fontSubtitle, rgb(185, 203, 232))
		lblHeaderBadge = 0
		// Queue card
		lblQueueTitle = createLabel("MODEL QUEUE", fontSection, rgb(242, 247, 255))
		lblCount = createLabel("0 MODELS QUEUED", fontBadge, rgb(115, 224, 255))
		lblQueueHint = createLabel("Drag files here", fontSmall, rgb(151, 166, 194))
		listModels = create("SysListView32", "", WS_CHILD|WS_VISIBLE|WS_BORDER|WS_VSCROLL|WS_TABSTOP|LVS_REPORT|LVS_SHOWSELALWAYS, 0, 0, 10, 10, ID_LIST)
		applyExplorerTheme(listModels)
		initModelTable()
		btnAddFiles = createButton("+  ADD FILES", ID_ADD_FILES, rgb(67, 126, 255), rgb(83, 63, 214), rgb(127, 174, 255))
		btnAddFolder = createButton("+  ADD FOLDER", ID_ADD_FOLDER, rgb(23, 190, 222), rgb(26, 126, 196), rgb(92, 224, 255))
		btnRemove = createButton("REMOVE", ID_REMOVE, rgb(255, 157, 73), rgb(198, 79, 42), rgb(255, 190, 122))
		btnClear = createButton("CLEAR", ID_CLEAR, rgb(238, 91, 122), rgb(161, 45, 85), rgb(255, 141, 165))
		// Settings card
		lblSettingsTitle = createLabel("RENDER SETUP", fontSection, rgb(242, 247, 255))
		lblQuality = createLabel("MAX QUALITY", fontBadge, rgb(119, 255, 190))
		lblOutputTitle = createLabel("Default output folder", fontBody, rgb(220, 229, 245))
		lblOutputHint = createLabel("Saved automatically. Batch renders never ask for a destination again.", fontSmall, rgb(151, 166, 194))
		edOutput = create("EDIT", settings.OutputFolder, WS_CHILD|WS_VISIBLE|WS_BORDER|ES_AUTOHSCROLL|ES_READONLY|WS_TABSTOP, 0, 0, 10, 10, ID_OUTPUT)
		applyExplorerTheme(edOutput)
		btnBrowseOutput = createButton("BROWSE", ID_BROWSE_OUT, rgb(34, 201, 222), rgb(38, 121, 196), rgb(105, 232, 255))
		lblView = createLabel("View", fontSmall, rgb(191, 204, 227))
		lblRes = createLabel("Resolution", fontSmall, rgb(191, 204, 227))
		lblAA = createLabel("Anti-aliasing", fontSmall, rgb(191, 204, 227))
		cbView = create("COMBOBOX", "", WS_CHILD|WS_VISIBLE|WS_TABSTOP|CBS_DROPDOWNLIST|WS_VSCROLL, 0, 0, 10, 10, ID_VIEW)
		addCombo(cbView, []string{"Auto Side", "Look along X", "Look along Y", "Look along Z"}, settings.View)
		applyExplorerTheme(cbView)
		cbRes = create("COMBOBOX", "", WS_CHILD|WS_VISIBLE|WS_TABSTOP|CBS_DROPDOWNLIST|WS_VSCROLL, 0, 0, 10, 10, ID_RES)
		addCombo(cbRes, []string{"2000 x 1000", "3000 x 1500", "4000 x 2000  •  MAX"}, settings.Resolution)
		applyExplorerTheme(cbRes)
		cbAA = create("COMBOBOX", "", WS_CHILD|WS_VISIBLE|WS_TABSTOP|CBS_DROPDOWNLIST|WS_VSCROLL, 0, 0, 10, 10, ID_AA)
		addCombo(cbAA, []string{"1x  •  Fast", "2x SSAA  •  MAX"}, settings.AA)
		applyExplorerTheme(cbAA)
		ckMirror = create("BUTTON", "Mirror horizontally", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_AUTOCHECKBOX, 0, 0, 10, 10, ID_MIRROR)
		ckOriginal = create("BUTTON", "Preserve OBJ vertex normals  (recommended)", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_AUTOCHECKBOX, 0, 0, 10, 10, ID_ORIG)
		ckOverwrite = create("BUTTON", "Overwrite existing PNGs", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_AUTOCHECKBOX, 0, 0, 10, 10, ID_OVERWRITE)
		setCheck(ckMirror, settings.Mirror)
		setCheck(ckOriginal, settings.PreserveNormals)
		setCheck(ckOverwrite, settings.Overwrite)
		updateQualityBadge()
		// Action bar
		lblStatusTitle = createLabel("STATUS", fontBadge, rgb(119, 224, 255))
		status = createLabel("", fontSmall, rgb(210, 221, 240))
		progress = create("msctls_progress32", "", WS_CHILD|WS_VISIBLE, 0, 0, 10, 10, 0)
		send(progress, PBM_SETRANGE32, 0, 1)
		send(progress, PBM_SETPOS, 0, 0)
		send(progress, PBM_SETBARCOLOR, 0, uintptr(rgb(64, 210, 255)))
		send(progress, PBM_SETBKCOLOR, 0, uintptr(rgb(30, 37, 59)))
		btnRender = createButton("RENDER", ID_RENDER, rgb(41, 221, 158), rgb(14, 134, 120), rgb(111, 255, 203))
		btnCancel = createButton("CANCEL", ID_CANCEL, rgb(239, 87, 116), rgb(159, 41, 79), rgb(255, 142, 166))
		btnOpenFolder = createButton("OPEN OUTPUT FOLDER", ID_OPEN_FOLDER, rgb(147, 92, 255), rgb(86, 55, 196), rgb(190, 151, 255))
		pEnableWindow.Call(uintptr(btnCancel), 0)
		refreshModelList()
		pDragAcceptFiles.Call(uintptr(hwnd), 1)
		var cr RECT
		pGetClientRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&cr)))
		layoutControls(cr.Right-cr.Left, cr.Bottom-cr.Top)
		applyDarkTitleBar(hwnd)
		return 0
	case WM_SIZE:
		cw := int32(loword(lParam))
		ch := int32(hiword(lParam))
		if cw > 0 && ch > 0 {
			layoutControls(cw, ch)
		}
		return 0
	case WM_GETMINMAXINFO:
		mmi := (*MINMAXINFO)(unsafe.Pointer(lParam))
		mmi.PtMinTrackSize = POINT{1160, 780}
		return 0
	case WM_ERASEBKGND:
		return 1
	case WM_PAINT:
		paintWindow(hwnd)
		return 0
	case WM_DRAWITEM:
		if lParam != 0 {
			drawButton((*DRAWITEMSTRUCT)(unsafe.Pointer(lParam)))
			return 1
		}
		return 0
	case WM_CTLCOLORSTATIC, WM_CTLCOLORBTN:
		hdc := syscall.Handle(wParam)
		child := syscall.Handle(lParam)
		c := rgb(219, 229, 247)
		if v, ok := staticColors[child]; ok {
			c = v
		}
		pSetTextColor.Call(uintptr(hdc), uintptr(c))
		// Runtime labels change text in place. Give them an opaque panel background
		// so the previous string is completely erased before the new one is painted.
		if child == lblQuality || child == status {
			pSetBkMode.Call(uintptr(hdc), 2) // OPAQUE
			pSetBkColor.Call(uintptr(hdc), uintptr(rgb(20, 27, 48)))
			return uintptr(brushPanel)
		}
		pSetBkMode.Call(uintptr(hdc), TRANSPARENT)
		return uintptr(brushTransparent)
	case WM_CTLCOLOREDIT:
		hdc := syscall.Handle(wParam)
		pSetTextColor.Call(uintptr(hdc), uintptr(rgb(231, 239, 252)))
		pSetBkColor.Call(uintptr(hdc), uintptr(rgb(31, 39, 63)))
		return uintptr(brushEdit)
	case WM_CTLCOLORLISTBOX:
		hdc := syscall.Handle(wParam)
		pSetTextColor.Call(uintptr(hdc), uintptr(rgb(222, 232, 248)))
		pSetBkColor.Call(uintptr(hdc), uintptr(rgb(25, 33, 56)))
		return uintptr(brushList)
	case WM_NOTIFY:
		if lParam != 0 {
			hdr := (*NMHDR)(unsafe.Pointer(lParam))
			if hdr.HwndFrom == listModels && int32(hdr.Code) == LVN_COLUMNCLICK {
				nm := (*NMLISTVIEW)(unsafe.Pointer(lParam))
				if nm.ISubItem == 0 || nm.ISubItem == 1 {
					sortModelTable(int(nm.ISubItem), true)
					refreshModelList()
				}
				return 0
			}
		}
		return 0
	case WM_COMMAND:
		id := loword(wParam)
		code := hiword(wParam)
		if code == CBN_SELCHANGE && (id == ID_RES || id == ID_AA) {
			updateQualityBadge()
			return 0
		}
		if code == BN_CLICKED {
			switch id {
			case ID_ADD_FILES:
				startAddFiles()
			case ID_ADD_FOLDER:
				startAddFolder()
			case ID_REMOVE:
				removeSelectedModels()
			case ID_CLEAR:
				if !rendering {
					models = nil
					refreshModelList()
					setText(status, "Queue cleared.")
				}
			case ID_BROWSE_OUT:
				startOutputFolder()
			case ID_RENDER:
				startBatchRender()
			case ID_CANCEL:
				cancelBatch()
			case ID_OPEN_FOLDER:
				openOutputFolder()
			case ID_MIRROR, ID_ORIG:
				// View/fidelity toggles do not change the image-quality badge.
			}
		}
		return 0
	case WM_DROPFILES:
		handleDrop(syscall.Handle(wParam))
		return 0
	case WM_ADD_FILES_DONE:
		finishAddFiles()
		return 0
	case WM_ADD_FOLDER_DONE:
		finishAddFolder()
		return 0
	case WM_OUT_FOLDER_DONE:
		finishOutputFolder()
		return 0
	case WM_BATCH_PROGRESS:
		updateBatchProgress()
		return 0
	case WM_BATCH_DONE:
		finishBatchRender()
		return 0
	case WM_DESTROY:
		saveSettings()
		for _, h := range []syscall.Handle{fontBody, fontTitle, fontSubtitle, fontSection, fontButton, fontSmall, fontBadge, brushPanel, brushEdit, brushList} {
			if h != 0 {
				pDeleteObject.Call(uintptr(h))
			}
		}
		pPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := pDefWindowProcW.Call(uintptr(hwnd), uintptr(msg), wParam, lParam)
	return r
}

func gui() int {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	pSetProcessDPIAware.Call()
	icc := INITCOMMONCONTROLSEX{DwSize: uint32(unsafe.Sizeof(INITCOMMONCONTROLSEX{})), DwICC: ICC_LISTVIEW_CLASSES | ICC_PROGRESS_CLASS}
	pInitCommonControlsEx.Call(uintptr(unsafe.Pointer(&icc)))
	settings = loadSettings()
	modelSortColumn = 0
	modelSortAscending = true
	_ = os.MkdirAll(settings.OutputFolder, 0755)
	inst, _, _ := pGetModuleHandleW.Call(0)
	cur, _, _ := pLoadCursorW.Call(0, IDC_ARROW)
	className := w("OBJ2PNGStudioProRelease")
	wndProcPtr := syscall.NewCallback(wndProc)
	bigIcon := loadEmbeddedAppIcon(64)
	smallIcon := loadEmbeddedAppIcon(20)
	wc := WNDCLASSEX{cbSize: uint32(unsafe.Sizeof(WNDCLASSEX{})), lpfnWndProc: wndProcPtr, hInstance: syscall.Handle(inst), hIcon: bigIcon, hCursor: syscall.Handle(cur), hbrBackground: 0, lpszClassName: className, hIconSm: smallIcon}
	atom, _, regErr := pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	runtime.KeepAlive(className)
	runtime.KeepAlive(wndProcPtr)
	if atom == 0 {
		if errno, ok := regErr.(syscall.Errno); !ok || errno != 1410 {
			return 1
		}
	}
	title := w("OBJ2PNG Studio Pro v1.0.0")
	className2 := w("OBJ2PNGStudioProRelease")
	style := uintptr(WS_OVERLAPPED | WS_CAPTION | WS_SYSMENU | WS_MINIMIZEBOX | WS_MAXIMIZEBOX | WS_THICKFRAME)
	h, _, _ := pCreateWindowExW.Call(0, uintptr(unsafe.Pointer(className2)), uintptr(unsafe.Pointer(title)), style, uintptr(CW_USEDEFAULT), uintptr(CW_USEDEFAULT), 1240, 860, 0, 0, inst, 0)
	runtime.KeepAlive(title)
	runtime.KeepAlive(className2)
	if h == 0 {
		return 2
	}
	hwndMain = syscall.Handle(h)
	applyEmbeddedWindowIcons(hwndMain)
	pShowWindow.Call(h, SW_SHOW)
	pUpdateWindow.Call(h)
	var msg MSG
	for {
		r, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(r) == -1 {
			return 3
		}
		if r == 0 {
			break
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}
	return 0
}
func main() { os.Exit(gui()) }
