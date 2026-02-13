//go:build windows

package wininput

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"clicker/internal/core/autoclicker"
)

const (
	whKeyboardLL = 13
	whMouseLL    = 14

	wmQuit        = 0x0012
	wmKeyDown     = 0x0100
	wmKeyUp       = 0x0101
	wmSysKeyDown  = 0x0104
	wmSysKeyUp    = 0x0105
	wmLButtonDown = 0x0201
	wmLButtonUp   = 0x0202
	wmRButtonDown = 0x0204
	wmRButtonUp   = 0x0205
	wmMButtonDown = 0x0207
	wmMButtonUp   = 0x0208
	wmXButtonDown = 0x020B
	wmXButtonUp   = 0x020C

	xButton1 = 0x0001
	xButton2 = 0x0002

	llmhfInjected        = 0x00000001
	llkhfInjected        = 0x00000010
	llkhfLowerILInjected = 0x00000002

	inputMouse           = 0
	mouseeventfMove      = 0x0001
	mouseeventfLeftDown  = 0x0002
	mouseeventfLeftUp    = 0x0004
	globalSourceIdentity = "windows-global"
)

var (
	user32 = syscall.NewLazyDLL("user32.dll")

	procSetWindowsHookExW   = user32.NewProc("SetWindowsHookExW")
	procUnhookWindowsHookEx = user32.NewProc("UnhookWindowsHookEx")
	procCallNextHookEx      = user32.NewProc("CallNextHookEx")
	procGetMessageW         = user32.NewProc("GetMessageW")
	procTranslateMessage    = user32.NewProc("TranslateMessage")
	procDispatchMessageW    = user32.NewProc("DispatchMessageW")
	procPostThreadMessageW  = user32.NewProc("PostThreadMessageW")
	procSendInput           = user32.NewProc("SendInput")
	procGetAsyncKeyState    = user32.NewProc("GetAsyncKeyState")

	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	procGetCurrentThreadID = kernel32.NewProc("GetCurrentThreadId")

	mouseHookCallback    = syscall.NewCallback(mouseLLCallback)
	keyboardHookCallback = syscall.NewCallback(keyboardLLCallback)

	activeRuntime atomic.Pointer[Runtime]
)

type point struct {
	X int32
	Y int32
}

type mouseLLHookStruct struct {
	Pt          point
	MouseData   uint32
	Flags       uint32
	Time        uint32
	DwExtraInfo uintptr
}

type keyboardLLHookStruct struct {
	VkCode      uint32
	ScanCode    uint32
	Flags       uint32
	Time        uint32
	DwExtraInfo uintptr
}

type message struct {
	Hwnd     uintptr
	Message  uint32
	WParam   uintptr
	LParam   uintptr
	Time     uint32
	Pt       point
	LPrivate uint32
}

type mouseInput struct {
	Dx          int32
	Dy          int32
	MouseData   uint32
	DwFlags     uint32
	Time        uint32
	DwExtraInfo uintptr
}

type input struct {
	Type uint32
	Mi   mouseInput
}

type windowsInjector struct{}

func (i *windowsInjector) WriteEvents(events ...autoclicker.Event) error {
	inputs := make([]input, 0, len(events))
	var moveX int32
	var moveY int32

	flushMove := func() {
		if moveX == 0 && moveY == 0 {
			return
		}
		inputs = append(inputs, input{
			Type: inputMouse,
			Mi: mouseInput{
				Dx:      moveX,
				Dy:      moveY,
				DwFlags: mouseeventfMove,
			},
		})
		moveX = 0
		moveY = 0
	}

	for _, event := range events {
		switch event.Type {
		case autoclicker.EventTypeRel:
			switch event.Code {
			case autoclicker.RelXCode:
				moveX += event.Value
			case autoclicker.RelYCode:
				moveY += event.Value
			}
		case autoclicker.EventTypeSyn:
			if event.Code == autoclicker.SynReportCode {
				flushMove()
			}
		case autoclicker.EventTypeKey:
			if event.Code != autoclicker.LeftButtonCode {
				continue
			}
			flushMove()

			var flags uint32
			switch event.Value {
			case 1:
				flags = mouseeventfLeftDown
			case 0:
				flags = mouseeventfLeftUp
			default:
				continue
			}

			inputs = append(inputs, input{
				Type: inputMouse,
				Mi: mouseInput{
					DwFlags: flags,
				},
			})
		default:
			continue
		}
	}
	flushMove()

	if len(inputs) == 0 {
		return nil
	}

	sent, _, callErr := procSendInput.Call(
		uintptr(len(inputs)),
		uintptr(unsafe.Pointer(&inputs[0])),
		unsafe.Sizeof(inputs[0]),
	)
	if sent != uintptr(len(inputs)) {
		if callErr != nil && callErr != syscall.Errno(0) {
			return callErr
		}
		return fmt.Errorf("SendInput sent %d of %d inputs", sent, len(inputs))
	}
	return nil
}

func (i *windowsInjector) Close() error {
	return nil
}

type Runtime struct {
	service *autoclicker.Service
	logger  autoclicker.Logger

	stopOnce sync.Once
	stopCh   chan struct{}

	threadID atomic.Uint32
	loopMu   sync.Mutex
	loopDone chan struct{}

	captureMu sync.Mutex
	captureCh chan uint16
}

func NewRuntime(cfg RuntimeConfig, logger autoclicker.Logger) (*Runtime, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger is nil")
	}

	service, err := autoclicker.NewService(
		autoclicker.Config{
			TriggerCode:    cfg.TriggerCode,
			ToggleCode:     cfg.ToggleCode,
			TriggerSources: map[string]struct{}{globalSourceIdentity: {}},
			ToggleSources:  map[string]struct{}{globalSourceIdentity: {}},
			GrabSources:    nil,
			GrabEnabled:    false,
			CPS:            cfg.CPS,
			ClickDown:      cfg.ClickDown,
			JitterPixels:   cfg.JitterPixels,
			StartEnabled:   cfg.StartEnabled,
		},
		&windowsInjector{},
		logger,
	)
	if err != nil {
		return nil, err
	}

	return &Runtime{
		service:  service,
		logger:   logger,
		stopCh:   make(chan struct{}),
		loopDone: closedSignalChan(),
	}, nil
}

func (r *Runtime) Start() error {
	if !activeRuntime.CompareAndSwap(nil, r) {
		return fmt.Errorf("windows runtime is already active")
	}

	r.loopMu.Lock()
	r.loopDone = make(chan struct{})
	r.loopMu.Unlock()

	r.service.Start()

	ready := make(chan error, 1)
	go r.hookLoop(ready)

	if err := <-ready; err != nil {
		r.Stop()
		return err
	}
	return nil
}

func (r *Runtime) Stop() {
	r.stopOnce.Do(func() {
		close(r.stopCh)
		threadID := r.threadID.Load()
		if threadID != 0 {
			_, _, _ = procPostThreadMessageW.Call(uintptr(threadID), uintptr(wmQuit), 0, 0)
		}

		r.loopMu.Lock()
		done := r.loopDone
		r.loopMu.Unlock()
		if done != nil {
			<-done
		}

		r.service.Stop()
		activeRuntime.CompareAndSwap(r, nil)
	})
}

func (r *Runtime) SetEnabled(enabled bool) {
	r.service.SetEnabled(enabled)
}

func (r *Runtime) IsEnabled() bool {
	return r.service.IsEnabled()
}

func (r *Runtime) SetCPS(cps float64) error {
	return r.service.SetCPS(cps)
}

func (r *Runtime) SetJitter(pixels int) error {
	return r.service.SetJitter(pixels)
}

func (r *Runtime) SetTriggerCode(code uint16) {
	r.service.SetTriggerCode(code)
}

func (r *Runtime) SetToggleCode(code uint16) {
	r.service.SetToggleCode(code)
}

func (r *Runtime) CaptureNextKeyCode(timeout time.Duration) (uint16, error) {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	waitCh := make(chan uint16, 1)

	r.captureMu.Lock()
	if r.captureCh != nil {
		r.captureMu.Unlock()
		return 0, fmt.Errorf("key capture already in progress")
	}
	r.captureCh = waitCh
	r.captureMu.Unlock()

	defer func() {
		r.captureMu.Lock()
		if r.captureCh == waitCh {
			r.captureCh = nil
		}
		r.captureMu.Unlock()
	}()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case code := <-waitCh:
		return code, nil
	case <-r.stopCh:
		return 0, fmt.Errorf("runtime stopped")
	case <-timer.C:
		return 0, fmt.Errorf("timed out waiting for key/button input")
	}
}

func (r *Runtime) hookLoop(ready chan<- error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer func() {
		r.loopMu.Lock()
		done := r.loopDone
		r.loopMu.Unlock()
		if done != nil {
			close(done)
		}
	}()
	defer activeRuntime.CompareAndSwap(r, nil)

	threadID, _, _ := procGetCurrentThreadID.Call()
	r.threadID.Store(uint32(threadID))

	mouseHook, _, mouseErr := procSetWindowsHookExW.Call(uintptr(whMouseLL), mouseHookCallback, 0, 0)
	if mouseHook == 0 {
		ready <- fmt.Errorf("failed to install mouse hook: %w", mouseErr)
		return
	}
	defer func() {
		_, _, _ = procUnhookWindowsHookEx.Call(mouseHook)
	}()

	keyboardHook, _, keyboardErr := procSetWindowsHookExW.Call(uintptr(whKeyboardLL), keyboardHookCallback, 0, 0)
	if keyboardHook == 0 {
		ready <- fmt.Errorf("failed to install keyboard hook: %w", keyboardErr)
		return
	}
	defer func() {
		_, _, _ = procUnhookWindowsHookEx.Call(keyboardHook)
	}()

	ready <- nil

	var msg message
	for {
		ret, _, callErr := procGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		switch int32(ret) {
		case -1:
			r.logger.Warn("Windows message loop failed", "err", callErr)
			return
		case 0:
			return
		default:
			_, _, _ = procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
			_, _, _ = procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
		}
	}
}

func mouseLLCallback(code int, wParam uintptr, lParam uintptr) uintptr {
	if code >= 0 {
		if r := activeRuntime.Load(); r != nil {
			r.handleMouseHook(wParam, lParam)
		}
	}
	ret, _, _ := procCallNextHookEx.Call(0, uintptr(code), wParam, lParam)
	return ret
}

func keyboardLLCallback(code int, wParam uintptr, lParam uintptr) uintptr {
	if code >= 0 {
		if r := activeRuntime.Load(); r != nil {
			r.handleKeyboardHook(wParam, lParam)
		}
	}
	ret, _, _ := procCallNextHookEx.Call(0, uintptr(code), wParam, lParam)
	return ret
}

func (r *Runtime) handleMouseHook(wParam uintptr, lParam uintptr) {
	if lParam == 0 {
		return
	}

	event := (*mouseLLHookStruct)(unsafe.Pointer(lParam))
	if event.Flags&llmhfInjected != 0 {
		return
	}

	var (
		code  uint16
		value int32
		ok    bool
	)

	switch uint32(wParam) {
	case wmLButtonDown:
		code, value, ok = CodeBTNLeft, 1, true
	case wmLButtonUp:
		code, value, ok = CodeBTNLeft, 0, true
	case wmRButtonDown:
		code, value, ok = CodeBTNRight, 1, true
	case wmRButtonUp:
		code, value, ok = CodeBTNRight, 0, true
	case wmMButtonDown:
		code, value, ok = CodeBTNMiddle, 1, true
	case wmMButtonUp:
		code, value, ok = CodeBTNMiddle, 0, true
	case wmXButtonDown:
		code, value, ok = xButtonCode(event.MouseData), 1, true
	case wmXButtonUp:
		code, value, ok = xButtonCode(event.MouseData), 0, true
	}
	if !ok || code == 0 {
		return
	}

	if value == 1 {
		r.publishCapturedCode(code)
	}
	_ = r.service.SubmitEvent(globalSourceIdentity, autoclicker.Event{
		Type:  autoclicker.EventTypeKey,
		Code:  code,
		Value: value,
	})
}

func (r *Runtime) handleKeyboardHook(wParam uintptr, lParam uintptr) {
	if lParam == 0 {
		return
	}

	event := (*keyboardLLHookStruct)(unsafe.Pointer(lParam))
	if event.Flags&llkhfInjected != 0 || event.Flags&llkhfLowerILInjected != 0 {
		return
	}

	code, ok := CodeFromVK(event.VkCode, event.Flags, event.ScanCode)
	if !ok {
		return
	}

	var value int32
	switch uint32(wParam) {
	case wmKeyDown, wmSysKeyDown:
		value = 1
	case wmKeyUp, wmSysKeyUp:
		value = 0
	default:
		return
	}

	if value == 1 {
		r.publishCapturedCode(code)
	}
	_ = r.service.SubmitEvent(globalSourceIdentity, autoclicker.Event{
		Type:  autoclicker.EventTypeKey,
		Code:  code,
		Value: value,
	})
}

func xButtonCode(mouseData uint32) uint16 {
	switch uint16(mouseData >> 16) {
	case xButton1:
		return CodeBTNSide
	case xButton2:
		return CodeBTNExtra
	default:
		return 0
	}
}

func (r *Runtime) publishCapturedCode(code uint16) {
	r.captureMu.Lock()
	ch := r.captureCh
	r.captureMu.Unlock()
	if ch == nil {
		return
	}
	select {
	case ch <- code:
	default:
	}
}

func ListInputDevices() ([]DeviceInfo, error) {
	return []DeviceInfo{
		{
			Path:      "global",
			Name:      "Windows Global Input",
			IsVirtual: false,
			IsPointer: true,
		},
	}, nil
}

func CaptureNextKeyCode(timeout time.Duration) (uint16, error) {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	codes := CaptureCandidateCodes()
	if len(codes) == 0 {
		return 0, fmt.Errorf("no capturable key/button codes configured")
	}

	state := make(map[uint16]bool, len(codes))
	for _, code := range codes {
		state[code] = isCodeDown(code)
	}

	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(2 * time.Millisecond)
	defer ticker.Stop()

	for {
		for _, code := range codes {
			down := isCodeDown(code)
			wasDown := state[code]
			state[code] = down
			if down && !wasDown {
				return code, nil
			}
		}

		if time.Now().After(deadline) {
			return 0, fmt.Errorf("timed out waiting for key/button input")
		}

		<-ticker.C
	}
}

func isCodeDown(code uint16) bool {
	vk, ok := CodeToVK(code)
	if !ok {
		return false
	}
	state, _, _ := procGetAsyncKeyState.Call(uintptr(vk))
	return uint16(state)&0x8000 != 0
}

func closedSignalChan() chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}
