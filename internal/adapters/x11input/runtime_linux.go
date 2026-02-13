//go:build linux

package x11input

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"clicker/internal/adapters/linuxinput"
	"clicker/internal/core/autoclicker"

	"github.com/BurntSushi/xgb"
	"github.com/BurntSushi/xgb/xproto"
	"github.com/BurntSushi/xgb/xtest"
	"github.com/BurntSushi/xgbutil"
	"github.com/BurntSushi/xgbutil/keybind"
)

type codeBinding struct {
	code     uint16
	keycodes []xproto.Keycode
	buttons  []xproto.Button
}

type Runtime struct {
	xu      *xgbutil.XUtil
	conn    *xgb.Conn
	rootWin xproto.Window

	service *autoclicker.Service
	logger  autoclicker.Logger

	mu             sync.RWMutex
	triggerCode    uint16
	toggleCode     uint16
	triggerBinding codeBinding
	toggleBinding  codeBinding
	keyToCode      map[xproto.Keycode]uint16
	buttonToCode   map[xproto.Button]uint16

	grabbedKeys    []xproto.Keycode
	grabbedButtons []xproto.Button

	injectMu sync.Mutex

	stopOnce sync.Once
	stopCh   chan struct{}
	doneCh   chan struct{}
}

type x11Injector struct {
	r *Runtime
}

func (i *x11Injector) WriteEvents(events ...autoclicker.Event) error {
	i.r.injectMu.Lock()
	defer i.r.injectMu.Unlock()

	var (
		moveX int32
		moveY int32
		dirty bool
	)

	flushMove := func() error {
		if moveX == 0 && moveY == 0 {
			return nil
		}

		query, err := xproto.QueryPointer(i.r.conn, i.r.rootWin).Reply()
		if err != nil {
			return err
		}
		nextX := clampInt32ToInt16(int32(query.RootX) + moveX)
		nextY := clampInt32ToInt16(int32(query.RootY) + moveY)
		if err := xproto.WarpPointerChecked(
			i.r.conn,
			xproto.WindowNone,
			i.r.rootWin,
			0,
			0,
			0,
			0,
			nextX,
			nextY,
		).Check(); err != nil {
			return err
		}
		moveX = 0
		moveY = 0
		dirty = true
		return nil
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
				if err := flushMove(); err != nil {
					return err
				}
			}
		case autoclicker.EventTypeKey:
			if event.Code != autoclicker.LeftButtonCode {
				continue
			}
			if err := flushMove(); err != nil {
				return err
			}

			var eventType byte
			switch event.Value {
			case 1:
				eventType = xproto.ButtonPress
			case 0:
				eventType = xproto.ButtonRelease
			default:
				continue
			}

			if err := xtest.FakeInputChecked(
				i.r.conn,
				eventType,
				byte(xproto.ButtonIndex1),
				xproto.TimeCurrentTime,
				i.r.rootWin,
				0,
				0,
				0,
			).Check(); err != nil {
				return err
			}
			dirty = true
		}
	}

	if err := flushMove(); err != nil {
		return err
	}
	if dirty {
		i.r.conn.Sync()
	}
	return nil
}

func (i *x11Injector) Close() error {
	return nil
}

func NewRuntime(cfg RuntimeConfig, logger autoclicker.Logger) (*Runtime, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger is nil")
	}

	xu, err := xgbutil.NewConn()
	if err != nil {
		return nil, err
	}
	conn := xu.Conn()
	if conn == nil {
		return nil, fmt.Errorf("failed to open X11 connection")
	}

	if err := xtest.Init(conn); err != nil {
		conn.Close()
		return nil, err
	}
	keybind.Initialize(xu)

	r := &Runtime{
		xu:      xu,
		conn:    conn,
		rootWin: xu.RootWin(),
		logger:  logger,
		stopCh:  make(chan struct{}),
		doneCh:  make(chan struct{}),
	}

	service, err := autoclicker.NewService(
		autoclicker.Config{
			TriggerCode:    cfg.TriggerCode,
			ToggleCode:     cfg.ToggleCode,
			TriggerSources: map[string]struct{}{"x11-global": {}},
			ToggleSources:  map[string]struct{}{"x11-global": {}},
			GrabSources:    nil,
			GrabEnabled:    false,
			CPS:            cfg.CPS,
			ClickDown:      cfg.ClickDown,
			JitterPixels:   cfg.JitterPixels,
			StartEnabled:   cfg.StartEnabled,
		},
		&x11Injector{r: r},
		logger,
	)
	if err != nil {
		conn.Close()
		return nil, err
	}
	r.service = service

	if err := r.applyBindings(cfg.TriggerCode, cfg.ToggleCode); err != nil {
		r.service.Stop()
		conn.Close()
		return nil, err
	}

	return r, nil
}

func (r *Runtime) Start() error {
	r.service.Start()
	go r.eventLoop()
	return nil
}

func (r *Runtime) Stop() {
	r.stopOnce.Do(func() {
		close(r.stopCh)

		r.mu.Lock()
		r.ungrabAllLocked()
		if r.conn != nil {
			r.conn.Close()
		}
		r.mu.Unlock()

		<-r.doneCh
		r.service.Stop()
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
	r.mu.RLock()
	toggle := r.toggleCode
	r.mu.RUnlock()
	if err := r.applyBindings(code, toggle); err != nil {
		r.logger.Warn("Failed to update trigger binding", "err", err)
	}
}

func (r *Runtime) SetToggleCode(code uint16) {
	r.mu.RLock()
	trigger := r.triggerCode
	r.mu.RUnlock()
	if err := r.applyBindings(trigger, code); err != nil {
		r.logger.Warn("Failed to update toggle binding", "err", err)
	}
}

func (r *Runtime) CaptureNextKeyCode(timeout time.Duration) (uint16, error) {
	return 0, fmt.Errorf("live capture unavailable on x11 runtime")
}

func (r *Runtime) eventLoop() {
	defer close(r.doneCh)

	for {
		event, xerr := r.conn.WaitForEvent()
		if xerr != nil {
			select {
			case <-r.stopCh:
				return
			default:
			}
			r.logger.Warn("X11 event error", "err", xerr)
			continue
		}
		if event == nil {
			return
		}

		switch ev := event.(type) {
		case xproto.KeyPressEvent:
			if code, ok := r.lookupKeyCode(ev.Detail); ok {
				r.service.SubmitEvent("x11-global", autoclicker.Event{Type: autoclicker.EventTypeKey, Code: code, Value: 1})
			}
			_ = xproto.AllowEventsChecked(r.conn, xproto.AllowReplayKeyboard, xproto.TimeCurrentTime).Check()
		case xproto.KeyReleaseEvent:
			if code, ok := r.lookupKeyCode(ev.Detail); ok {
				r.service.SubmitEvent("x11-global", autoclicker.Event{Type: autoclicker.EventTypeKey, Code: code, Value: 0})
			}
			_ = xproto.AllowEventsChecked(r.conn, xproto.AllowReplayKeyboard, xproto.TimeCurrentTime).Check()
		case xproto.ButtonPressEvent:
			if code, ok := r.lookupButtonCode(ev.Detail); ok {
				r.service.SubmitEvent("x11-global", autoclicker.Event{Type: autoclicker.EventTypeKey, Code: code, Value: 1})
			}
			_ = xproto.AllowEventsChecked(r.conn, xproto.AllowReplayPointer, xproto.TimeCurrentTime).Check()
		case xproto.ButtonReleaseEvent:
			if code, ok := r.lookupButtonCode(ev.Detail); ok {
				r.service.SubmitEvent("x11-global", autoclicker.Event{Type: autoclicker.EventTypeKey, Code: code, Value: 0})
			}
			_ = xproto.AllowEventsChecked(r.conn, xproto.AllowReplayPointer, xproto.TimeCurrentTime).Check()
		}
	}
}

func (r *Runtime) lookupKeyCode(key xproto.Keycode) (uint16, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	code, ok := r.keyToCode[key]
	return code, ok
}

func (r *Runtime) lookupButtonCode(button xproto.Button) (uint16, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	code, ok := r.buttonToCode[button]
	return code, ok
}

func (r *Runtime) applyBindings(triggerCode, toggleCode uint16) error {
	triggerBinding, err := r.resolveBinding(triggerCode)
	if err != nil {
		return fmt.Errorf("trigger binding: %w", err)
	}
	toggleBinding, err := r.resolveBinding(toggleCode)
	if err != nil {
		return fmt.Errorf("toggle binding: %w", err)
	}

	keyToCode := make(map[xproto.Keycode]uint16)
	for _, key := range triggerBinding.keycodes {
		keyToCode[key] = triggerCode
	}
	for _, key := range toggleBinding.keycodes {
		if existing, ok := keyToCode[key]; ok && existing != toggleCode {
			return fmt.Errorf("trigger and toggle resolve to same X11 keycode")
		}
		keyToCode[key] = toggleCode
	}

	buttonToCode := make(map[xproto.Button]uint16)
	for _, button := range triggerBinding.buttons {
		buttonToCode[button] = triggerCode
	}
	for _, button := range toggleBinding.buttons {
		if existing, ok := buttonToCode[button]; ok && existing != toggleCode {
			return fmt.Errorf("trigger and toggle resolve to same X11 mouse button")
		}
		buttonToCode[button] = toggleCode
	}

	keys := make([]xproto.Keycode, 0, len(keyToCode))
	for key := range keyToCode {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })

	buttons := make([]xproto.Button, 0, len(buttonToCode))
	for button := range buttonToCode {
		buttons = append(buttons, button)
	}
	sort.Slice(buttons, func(i, j int) bool { return buttons[i] < buttons[j] })

	r.mu.Lock()
	defer r.mu.Unlock()

	r.ungrabAllLocked()
	if err := r.grabAllLocked(keys, buttons); err != nil {
		r.ungrabAllLocked()
		return err
	}

	r.triggerCode = triggerCode
	r.toggleCode = toggleCode
	r.triggerBinding = triggerBinding
	r.toggleBinding = toggleBinding
	r.keyToCode = keyToCode
	r.buttonToCode = buttonToCode
	if r.service != nil {
		r.service.SetTriggerCode(triggerCode)
		r.service.SetToggleCode(toggleCode)
	}
	return nil
}

func (r *Runtime) grabAllLocked(keys []xproto.Keycode, buttons []xproto.Button) error {
	for _, key := range keys {
		if err := xproto.GrabKeyChecked(
			r.conn,
			false,
			r.rootWin,
			xproto.ModMaskAny,
			key,
			xproto.GrabModeAsync,
			xproto.GrabModeAsync,
		).Check(); err != nil {
			return err
		}
		r.grabbedKeys = append(r.grabbedKeys, key)
	}

	for _, button := range buttons {
		if err := xproto.GrabButtonChecked(
			r.conn,
			false,
			r.rootWin,
			xproto.EventMaskButtonPress|xproto.EventMaskButtonRelease,
			xproto.GrabModeAsync,
			xproto.GrabModeAsync,
			xproto.WindowNone,
			xproto.CursorNone,
			byte(button),
			xproto.ModMaskAny,
		).Check(); err != nil {
			return err
		}
		r.grabbedButtons = append(r.grabbedButtons, button)
	}
	return nil
}

func (r *Runtime) ungrabAllLocked() {
	for _, key := range r.grabbedKeys {
		xproto.UngrabKey(r.conn, key, r.rootWin, xproto.ModMaskAny)
	}
	for _, button := range r.grabbedButtons {
		xproto.UngrabButton(r.conn, byte(button), r.rootWin, xproto.ModMaskAny)
	}
	r.grabbedKeys = nil
	r.grabbedButtons = nil
}

func (r *Runtime) resolveBinding(code uint16) (codeBinding, error) {
	if button, ok := codeToXButton(code); ok {
		return codeBinding{code: code, buttons: []xproto.Button{button}}, nil
	}

	keyName, ok := linuxCodeToXKeyString(code)
	if !ok {
		return codeBinding{}, fmt.Errorf("unsupported X11 key code %s", linuxinput.FormatCodeName(code))
	}

	keycodes := keybind.StrToKeycodes(r.xu, keyName)
	if len(keycodes) == 0 {
		return codeBinding{}, fmt.Errorf("failed to resolve X11 key %q", keyName)
	}

	uniq := make(map[xproto.Keycode]struct{}, len(keycodes))
	for _, keycode := range keycodes {
		uniq[keycode] = struct{}{}
	}
	result := make([]xproto.Keycode, 0, len(uniq))
	for key := range uniq {
		result = append(result, key)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return codeBinding{code: code, keycodes: result}, nil
}

func ListInputDevices() ([]DeviceInfo, error) {
	return []DeviceInfo{
		{
			Path:      "x11-global",
			Name:      "X11 Global Input",
			IsVirtual: false,
			IsPointer: true,
		},
	}, nil
}

func CaptureNextKeyCode(timeout time.Duration) (uint16, error) {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	xu, err := xgbutil.NewConn()
	if err != nil {
		return 0, err
	}
	conn := xu.Conn()
	root := xu.RootWin()
	keybind.Initialize(xu)

	defer conn.Close()
	defer xproto.UngrabPointer(conn, xproto.TimeCurrentTime)
	defer xproto.UngrabKeyboard(conn, xproto.TimeCurrentTime)

	if reply, err := xproto.GrabKeyboard(
		conn,
		false,
		root,
		xproto.TimeCurrentTime,
		xproto.GrabModeAsync,
		xproto.GrabModeAsync,
	).Reply(); err != nil {
		return 0, err
	} else if reply.Status != xproto.GrabStatusSuccess {
		return 0, fmt.Errorf("failed to grab keyboard (status=%d)", reply.Status)
	}

	if reply, err := xproto.GrabPointer(
		conn,
		false,
		root,
		xproto.EventMaskButtonPress|xproto.EventMaskButtonRelease,
		xproto.GrabModeAsync,
		xproto.GrabModeAsync,
		xproto.WindowNone,
		xproto.CursorNone,
		xproto.TimeCurrentTime,
	).Reply(); err != nil {
		return 0, err
	} else if reply.Status != xproto.GrabStatusSuccess {
		return 0, fmt.Errorf("failed to grab pointer (status=%d)", reply.Status)
	}

	deadline := time.Now().Add(timeout)
	for {
		event, xerr := conn.PollForEvent()
		if xerr != nil {
			return 0, xerr
		}
		if event == nil {
			if time.Now().After(deadline) {
				return 0, fmt.Errorf("timed out waiting for key/button input")
			}
			time.Sleep(2 * time.Millisecond)
			continue
		}

		switch ev := event.(type) {
		case xproto.ButtonPressEvent:
			if code, ok := xButtonToCode(ev.Detail); ok {
				return code, nil
			}
		case xproto.KeyPressEvent:
			lookup := keybind.LookupString(xu, ev.State, ev.Detail)
			if code, ok := xLookupStringToLinuxCode(lookup); ok {
				return code, nil
			}
		}
	}
}

func codeToXButton(code uint16) (xproto.Button, bool) {
	switch linuxinput.FormatCodeName(code) {
	case "BTN_LEFT":
		return xproto.Button(xproto.ButtonIndex1), true
	case "BTN_MIDDLE":
		return xproto.Button(xproto.ButtonIndex2), true
	case "BTN_RIGHT":
		return xproto.Button(xproto.ButtonIndex3), true
	case "BTN_SIDE", "BTN_BACK":
		return xproto.Button(8), true
	case "BTN_EXTRA", "BTN_FORWARD":
		return xproto.Button(9), true
	default:
		return 0, false
	}
}

func xButtonToCode(button xproto.Button) (uint16, bool) {
	switch byte(button) {
	case xproto.ButtonIndex1:
		return parseLinuxCode("BTN_LEFT")
	case xproto.ButtonIndex2:
		return parseLinuxCode("BTN_MIDDLE")
	case xproto.ButtonIndex3:
		return parseLinuxCode("BTN_RIGHT")
	case 8:
		return parseLinuxCode("BTN_SIDE")
	case 9:
		return parseLinuxCode("BTN_EXTRA")
	default:
		return 0, false
	}
}

func parseLinuxCode(name string) (uint16, bool) {
	code, err := linuxinput.ParseCode(name)
	if err != nil {
		return 0, false
	}
	return code, true
}

func linuxCodeToXKeyString(code uint16) (string, bool) {
	name := linuxinput.FormatCodeName(code)
	if !strings.HasPrefix(name, "KEY_") {
		return "", false
	}
	token := strings.TrimPrefix(name, "KEY_")

	switch token {
	case "ESC":
		return "Escape", true
	case "ENTER":
		return "Return", true
	case "TAB":
		return "Tab", true
	case "SPACE":
		return "space", true
	case "BACKSPACE":
		return "BackSpace", true
	case "LEFTSHIFT":
		return "Shift_L", true
	case "RIGHTSHIFT":
		return "Shift_R", true
	case "LEFTCTRL":
		return "Control_L", true
	case "RIGHTCTRL":
		return "Control_R", true
	case "LEFTALT":
		return "Alt_L", true
	case "RIGHTALT":
		return "Alt_R", true
	case "LEFTMETA":
		return "Super_L", true
	case "RIGHTMETA":
		return "Super_R", true
	case "CAPSLOCK":
		return "Caps_Lock", true
	case "NUMLOCK":
		return "Num_Lock", true
	case "SCROLLLOCK":
		return "Scroll_Lock", true
	case "PAGEUP":
		return "Page_Up", true
	case "PAGEDOWN":
		return "Page_Down", true
	case "INSERT":
		return "Insert", true
	case "DELETE":
		return "Delete", true
	case "HOME":
		return "Home", true
	case "END":
		return "End", true
	case "UP":
		return "Up", true
	case "DOWN":
		return "Down", true
	case "LEFT":
		return "Left", true
	case "RIGHT":
		return "Right", true
	case "MENU":
		return "Menu", true
	case "PAUSE":
		return "Pause", true
	case "MINUS":
		return "minus", true
	case "EQUAL":
		return "equal", true
	case "LEFTBRACE":
		return "bracketleft", true
	case "RIGHTBRACE":
		return "bracketright", true
	case "SEMICOLON":
		return "semicolon", true
	case "APOSTROPHE":
		return "apostrophe", true
	case "GRAVE":
		return "grave", true
	case "BACKSLASH":
		return "backslash", true
	case "COMMA":
		return "comma", true
	case "DOT":
		return "period", true
	case "SLASH":
		return "slash", true
	}

	if len(token) == 1 && token[0] >= 'A' && token[0] <= 'Z' {
		return strings.ToLower(token), true
	}
	if len(token) == 1 && token[0] >= '0' && token[0] <= '9' {
		return token, true
	}
	if strings.HasPrefix(token, "F") && len(token) > 1 && isDigits(token[1:]) {
		return token, true
	}
	if strings.HasPrefix(token, "KP") {
		suffix := strings.TrimPrefix(token, "KP")
		switch suffix {
		case "PLUS":
			return "KP_Add", true
		case "MINUS":
			return "KP_Subtract", true
		case "ASTERISK":
			return "KP_Multiply", true
		case "SLASH":
			return "KP_Divide", true
		case "DOT":
			return "KP_Decimal", true
		case "ENTER":
			return "KP_Enter", true
		}
		if len(suffix) == 1 && suffix[0] >= '0' && suffix[0] <= '9' {
			return "KP_" + suffix, true
		}
	}

	return "", false
}

func xLookupStringToLinuxCode(value string) (uint16, bool) {
	v := strings.TrimSpace(value)
	if v == "" {
		return 0, false
	}

	raw := strings.ToLower(v)
	keyName := ""
	if len(raw) == 1 && raw[0] >= 'a' && raw[0] <= 'z' {
		keyName = "KEY_" + strings.ToUpper(raw)
	} else if len(raw) == 1 && raw[0] >= '0' && raw[0] <= '9' {
		keyName = "KEY_" + strings.ToUpper(raw)
	} else if strings.HasPrefix(raw, "f") && len(raw) > 1 && isDigits(raw[1:]) {
		keyName = "KEY_" + strings.ToUpper(raw)
	} else if strings.HasPrefix(raw, "kp_") {
		suffix := strings.TrimPrefix(raw, "kp_")
		switch suffix {
		case "add":
			keyName = "KEY_KPPLUS"
		case "subtract":
			keyName = "KEY_KPMINUS"
		case "multiply":
			keyName = "KEY_KPASTERISK"
		case "divide":
			keyName = "KEY_KPSLASH"
		case "decimal":
			keyName = "KEY_KPDOT"
		case "enter":
			keyName = "KEY_KPENTER"
		default:
			if len(suffix) == 1 && suffix[0] >= '0' && suffix[0] <= '9' {
				keyName = "KEY_KP" + strings.ToUpper(suffix)
			}
		}
	} else {
		switch raw {
		case "escape":
			keyName = "KEY_ESC"
		case "return":
			keyName = "KEY_ENTER"
		case "tab":
			keyName = "KEY_TAB"
		case "space":
			keyName = "KEY_SPACE"
		case "backspace":
			keyName = "KEY_BACKSPACE"
		case "shift_l":
			keyName = "KEY_LEFTSHIFT"
		case "shift_r":
			keyName = "KEY_RIGHTSHIFT"
		case "control_l":
			keyName = "KEY_LEFTCTRL"
		case "control_r":
			keyName = "KEY_RIGHTCTRL"
		case "alt_l":
			keyName = "KEY_LEFTALT"
		case "alt_r":
			keyName = "KEY_RIGHTALT"
		case "super_l":
			keyName = "KEY_LEFTMETA"
		case "super_r":
			keyName = "KEY_RIGHTMETA"
		case "caps_lock":
			keyName = "KEY_CAPSLOCK"
		case "num_lock":
			keyName = "KEY_NUMLOCK"
		case "scroll_lock":
			keyName = "KEY_SCROLLLOCK"
		case "page_up":
			keyName = "KEY_PAGEUP"
		case "page_down":
			keyName = "KEY_PAGEDOWN"
		case "insert":
			keyName = "KEY_INSERT"
		case "delete":
			keyName = "KEY_DELETE"
		case "home":
			keyName = "KEY_HOME"
		case "end":
			keyName = "KEY_END"
		case "up":
			keyName = "KEY_UP"
		case "down":
			keyName = "KEY_DOWN"
		case "left":
			keyName = "KEY_LEFT"
		case "right":
			keyName = "KEY_RIGHT"
		case "menu":
			keyName = "KEY_MENU"
		case "pause":
			keyName = "KEY_PAUSE"
		case "minus":
			keyName = "KEY_MINUS"
		case "equal":
			keyName = "KEY_EQUAL"
		case "bracketleft":
			keyName = "KEY_LEFTBRACE"
		case "bracketright":
			keyName = "KEY_RIGHTBRACE"
		case "semicolon":
			keyName = "KEY_SEMICOLON"
		case "apostrophe":
			keyName = "KEY_APOSTROPHE"
		case "grave":
			keyName = "KEY_GRAVE"
		case "backslash":
			keyName = "KEY_BACKSLASH"
		case "comma":
			keyName = "KEY_COMMA"
		case "period":
			keyName = "KEY_DOT"
		case "slash":
			keyName = "KEY_SLASH"
		}
	}

	if keyName == "" {
		return 0, false
	}
	code, err := linuxinput.ParseCode(keyName)
	if err != nil {
		return 0, false
	}
	return code, true
}

func isDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func clampInt32ToInt16(value int32) int16 {
	if value < -32768 {
		return -32768
	}
	if value > 32767 {
		return 32767
	}
	return int16(value)
}
