package main

import (
	_ "embed"
	"errors"
	"fmt"
	"image/color"
	"math"
	"math/rand"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type clickerRuntime interface {
	SetEnabled(enabled bool)
	IsEnabled() bool
	SetCPS(cps float64) error
	SetJitter(pixels int) error
	SetTriggerCode(code uint16)
	SetToggleCode(code uint16)
	CaptureNextKeyCode(timeout time.Duration) (uint16, error)
	Stop()
}

//go:embed assets/JetBrainsMonoNerdFont-Regular.ttf
var jetBrainsMonoNerdFontRegular []byte

type clickerTheme struct {
	base fyne.Theme
	font fyne.Resource
}

func newClickerTheme() fyne.Theme {
	return &clickerTheme{
		base: theme.DarkTheme(),
		font: fyne.NewStaticResource("JetBrainsMonoNerdFont-Regular.ttf", jetBrainsMonoNerdFontRegular),
	}
}

func (t *clickerTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return color.NRGBA{R: 0x0d, G: 0x10, B: 0x14, A: 0xff}
	case theme.ColorNameHeaderBackground:
		return color.NRGBA{R: 0x12, G: 0x16, B: 0x1c, A: 0xff}
	case theme.ColorNameButton:
		return color.NRGBA{R: 0x1d, G: 0x23, B: 0x2c, A: 0xff}
	case theme.ColorNameDisabledButton:
		return color.NRGBA{R: 0x16, G: 0x1a, B: 0x20, A: 0xff}
	case theme.ColorNameInputBackground:
		return color.NRGBA{R: 0x13, G: 0x18, B: 0x1f, A: 0xff}
	case theme.ColorNameInputBorder, theme.ColorNameSeparator:
		return color.NRGBA{R: 0x2b, G: 0x33, B: 0x40, A: 0xff}
	case theme.ColorNamePrimary, theme.ColorNameHyperlink:
		return color.NRGBA{R: 0xff, G: 0x66, B: 0x66, A: 0xff}
	case theme.ColorNameFocus:
		return color.NRGBA{R: 0xff, G: 0x7a, B: 0x7a, A: 0x66}
	case theme.ColorNameHover:
		return color.NRGBA{R: 0xff, G: 0x7a, B: 0x7a, A: 0x22}
	case theme.ColorNamePressed:
		return color.NRGBA{R: 0xff, G: 0x7a, B: 0x7a, A: 0x40}
	case theme.ColorNameSelection:
		return color.NRGBA{R: 0xff, G: 0x66, B: 0x66, A: 0x44}
	case theme.ColorNameForeground:
		return color.NRGBA{R: 0xf2, G: 0xf4, B: 0xf8, A: 0xff}
	case theme.ColorNamePlaceHolder:
		return color.NRGBA{R: 0xa9, G: 0xb3, B: 0xc2, A: 0xff}
	case theme.ColorNameError:
		return color.NRGBA{R: 0xff, G: 0x82, B: 0x82, A: 0xff}
	case theme.ColorNameWarning:
		return color.NRGBA{R: 0xff, G: 0x9f, B: 0x5a, A: 0xff}
	case theme.ColorNameSuccess:
		return color.NRGBA{R: 0x7f, G: 0xd4, B: 0xa8, A: 0xff}
	}
	return t.base.Color(name, variant)
}

func (t *clickerTheme) Font(style fyne.TextStyle) fyne.Resource {
	if t.font != nil {
		return t.font
	}
	return t.base.Font(style)
}

func (t *clickerTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return t.base.Icon(name)
}

func (t *clickerTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNamePadding:
		return 8
	case theme.SizeNameInnerPadding:
		return 8
	case theme.SizeNameInputRadius:
		return 8
	}
	return t.base.Size(name)
}

func normalizeCodeName(raw string, fallback string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		value = fallback
	}
	code, err := parseTriggerCode(value)
	if err != nil {
		return strings.ToUpper(value)
	}
	return formatCodeName(code)
}

func displayCodeName(raw string) string {
	name := strings.ToUpper(strings.TrimSpace(raw))
	if name == "" {
		return "-"
	}

	switch name {
	case "BTN_LEFT":
		return "Mouse Left Button"
	case "BTN_RIGHT":
		return "Mouse Right Button"
	case "BTN_MIDDLE":
		return "Mouse Middle Button"
	case "BTN_SIDE", "BTN_EXTRA":
		return "Mouse Side Button"
	case "BTN_FORWARD":
		return "Mouse Forward Button"
	case "BTN_BACK":
		return "Mouse Back Button"
	}

	if strings.HasPrefix(name, "BTN_") {
		return "Mouse " + humanizeInputToken(strings.TrimPrefix(name, "BTN_"))
	}
	if strings.HasPrefix(name, "KEY_") {
		return "Keyboard " + humanizeInputToken(strings.TrimPrefix(name, "KEY_"))
	}
	return name
}

func humanizeInputToken(raw string) string {
	parts := strings.Split(raw, "_")
	words := make([]string, 0, len(parts)*2)
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		words = append(words, humanizeInputWord(part)...)
	}
	if len(words) == 0 {
		return raw
	}
	return strings.Join(words, " ")
}

func humanizeInputWord(raw string) []string {
	token := strings.ToUpper(strings.TrimSpace(raw))
	switch token {
	case "ALT":
		return []string{"Alt"}
	case "CTRL":
		return []string{"Ctrl"}
	case "SHIFT":
		return []string{"Shift"}
	case "ESC":
		return []string{"Esc"}
	case "ENTER":
		return []string{"Enter"}
	case "SPACE":
		return []string{"Space"}
	case "TAB":
		return []string{"Tab"}
	case "CAPSLOCK":
		return []string{"Caps", "Lock"}
	case "PAGEUP":
		return []string{"Page", "Up"}
	case "PAGEDOWN":
		return []string{"Page", "Down"}
	case "BACKSPACE":
		return []string{"Backspace"}
	case "DELETE":
		return []string{"Delete"}
	case "INSERT":
		return []string{"Insert"}
	case "HOME":
		return []string{"Home"}
	case "END":
		return []string{"End"}
	case "UP":
		return []string{"Up"}
	case "DOWN":
		return []string{"Down"}
	case "LEFT":
		return []string{"Left"}
	case "RIGHT":
		return []string{"Right"}
	}

	if strings.HasPrefix(token, "LEFT") && len(token) > len("LEFT") {
		return append([]string{"Left"}, humanizeInputWord(token[len("LEFT"):])...)
	}
	if strings.HasPrefix(token, "RIGHT") && len(token) > len("RIGHT") {
		return append([]string{"Right"}, humanizeInputWord(token[len("RIGHT"):])...)
	}
	if strings.HasPrefix(token, "KP") && len(token) > len("KP") {
		return append([]string{"Keypad"}, humanizeInputWord(token[len("KP"):])...)
	}
	if strings.HasPrefix(token, "F") && len(token) > 1 && isDigits(token[1:]) {
		return []string{token}
	}
	if len(token) == 1 {
		return []string{token}
	}
	return []string{strings.ToUpper(token[:1]) + strings.ToLower(token[1:])}
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

func runUI(baseCfg config) error {
	fApp := app.New()
	fApp.Settings().SetTheme(newClickerTheme())

	window := fApp.NewWindow("Auto-Clicker")
	window.Resize(fyne.NewSize(760, 470))
	window.SetFixedSize(true)
	window.CenterOnScreen()

	clamp := func(v, min, max float64) float64 {
		if v < min {
			return min
		}
		if v > max {
			return max
		}
		return v
	}

	startupEnabled := true
	settingsLoadWarning := ""

	minDefault := math.Max(1, baseCfg.cps-4)
	maxDefault := math.Max(minDefault, baseCfg.cps)
	jitterDefault := clamp(float64(baseCfg.jitter), 0, 12)

	triggerRaw := strings.TrimSpace(baseCfg.triggerRaw)
	if triggerRaw == "" {
		triggerRaw = "BTN_LEFT"
	}
	toggleRaw := strings.TrimSpace(baseCfg.toggleRaw)
	if toggleRaw == "" {
		toggleRaw = "BTN_EXTRA"
	}

	stored, err := loadUISettings()
	if err != nil {
		settingsLoadWarning = fmt.Sprintf("Failed to load saved settings: %v", err)
	} else if stored != nil {
		if stored.MinCPS > 0 {
			minDefault = clamp(stored.MinCPS, 1, 30)
		}
		if stored.MaxCPS > 0 {
			maxDefault = clamp(stored.MaxCPS, 1, 30)
		}
		if maxDefault < minDefault {
			maxDefault = minDefault
		}
		if stored.Jitter >= 0 {
			jitterDefault = clamp(float64(stored.Jitter), 0, 12)
		}
		if value := strings.TrimSpace(stored.Trigger); value != "" {
			if _, parseErr := parseTriggerCode(value); parseErr == nil {
				triggerRaw = value
			} else if settingsLoadWarning == "" {
				settingsLoadWarning = fmt.Sprintf("Saved trigger is invalid (%s); using default.", value)
			}
		}
		if value := strings.TrimSpace(stored.Toggle); value != "" {
			if _, parseErr := parseTriggerCode(value); parseErr == nil {
				toggleRaw = value
			} else if settingsLoadWarning == "" {
				settingsLoadWarning = fmt.Sprintf("Saved toggle is invalid (%s); using default.", value)
			}
		}
		startupEnabled = stored.Enabled
	}
	triggerRaw = normalizeCodeName(triggerRaw, "BTN_LEFT")
	toggleRaw = normalizeCodeName(toggleRaw, "BTN_EXTRA")

	minSlider := widget.NewSlider(1, 30)
	minSlider.Step = 0
	minSlider.SetValue(minDefault)

	maxSlider := widget.NewSlider(1, 30)
	maxSlider.Step = 0
	maxSlider.SetValue(maxDefault)

	jitterSlider := widget.NewSlider(0, 12)
	jitterSlider.Step = 0
	jitterSlider.SetValue(jitterDefault)

	minValue := widget.NewLabel("")
	maxValue := widget.NewLabel("")
	jitterValue := widget.NewLabel("")
	minValue.Alignment = fyne.TextAlignTrailing
	maxValue.Alignment = fyne.TextAlignTrailing
	jitterValue.Alignment = fyne.TextAlignTrailing
	minValue.TextStyle = fyne.TextStyle{Bold: true}
	maxValue.TextStyle = fyne.TextStyle{Bold: true}
	jitterValue.TextStyle = fyne.TextStyle{Bold: true}
	updateControlText := func() {
		minValue.SetText(fmt.Sprintf("%.2f", minSlider.Value))
		maxValue.SetText(fmt.Sprintf("%.2f", maxSlider.Value))
		jitterValue.SetText(fmt.Sprintf("%.2f px", jitterSlider.Value))
	}
	updateControlText()

	persistUISettings := func() {}

	minSlider.OnChanged = func(v float64) {
		if v > maxSlider.Value {
			maxSlider.SetValue(v)
		}
		updateControlText()
		persistUISettings()
	}
	maxSlider.OnChanged = func(v float64) {
		if v < minSlider.Value {
			minSlider.SetValue(v)
		}
		updateControlText()
		persistUISettings()
	}

	triggerCaptureBtn := widget.NewButton(displayCodeName(triggerRaw), nil)
	toggleCaptureBtn := widget.NewButton(displayCodeName(toggleRaw), nil)

	errorText := canvas.NewText("", nil)
	errorText.Color = theme.Color(theme.ColorNameError)
	if settingsLoadWarning != "" {
		errorText.Text = settingsLoadWarning
	}
	currentCPSText := widget.NewLabel("Current CPS: -")
	currentCPSText.TextStyle = fyne.TextStyle{Bold: true}
	logGrid := widget.NewTextGrid()
	logGrid.SetText("")
	logScroll := container.NewVScroll(logGrid)
	logScroll.SetMinSize(fyne.NewSize(0, 150))

	const maxUILogLines = 50
	var logMu sync.Mutex
	logLines := make([]string, 0, maxUILogLines)
	debugLogs := debugLogsEnabled()
	appendLogLine := func(line string) {
		if !debugLogs {
			return
		}

		line = strings.TrimSpace(line)
		if line == "" {
			return
		}

		logMu.Lock()
		logLines = append(logLines, line)
		if len(logLines) > maxUILogLines {
			logLines = logLines[len(logLines)-maxUILogLines:]
		}
		logText := strings.Join(logLines, "\n")
		logMu.Unlock()

		fyne.Do(func() {
			logGrid.SetText(logText)
			logScroll.ScrollToBottom()
		})
	}
	if settingsLoadWarning != "" {
		appendLogLine("WARNING " + settingsLoadWarning)
	}

	enableToggleBtn := widget.NewButton("Disabled", nil)
	enableToggleBtn.Importance = widget.HighImportance
	triggerCaptureBtn.Importance = widget.MediumImportance
	toggleCaptureBtn.Importance = widget.MediumImportance
	initProgress := widget.NewProgressBarInfinite()
	initProgress.Hide()

	setEnabledStateUI := func(enabled bool) {
		if enabled {
			enableToggleBtn.SetText("Enabled")
		} else {
			enableToggleBtn.SetText("Disabled")
		}
	}

	var stateMu sync.Mutex
	currentCfg := baseCfg
	var runningClicker clickerRuntime
	var runtimeStop chan struct{}
	initializing := false

	getState := func() (clickerRuntime, config, bool) {
		stateMu.Lock()
		defer stateMu.Unlock()
		return runningClicker, currentCfg, initializing
	}

	setInitializing := func(v bool) {
		stateMu.Lock()
		initializing = v
		stateMu.Unlock()
	}

	setCurrentCfg := func(cfg config) {
		stateMu.Lock()
		currentCfg = cfg
		stateMu.Unlock()
	}

	jitterSlider.OnChanged = func(v float64) {
		updateControlText()
		jitterPixels := int(math.Round(v))
		clicker, cfg, _ := getState()
		cfg.jitter = jitterPixels
		setCurrentCfg(cfg)
		if clicker != nil {
			if err := clicker.SetJitter(jitterPixels); err != nil {
				errorText.Text = err.Error()
				errorText.Refresh()
				appendLogLine("ERROR " + err.Error())
			}
		}
		persistUISettings()
	}

	setInitializingUI := func(v bool) {
		if v {
			initProgress.Show()
			return
		}
		initProgress.Hide()
	}

	stopRuntime := func() {
		stateMu.Lock()
		clicker := runningClicker
		stop := runtimeStop
		runningClicker = nil
		runtimeStop = nil
		stateMu.Unlock()

		if stop != nil {
			close(stop)
		}
		if clicker != nil {
			clicker.Stop()
		}
	}

	runRuntimeLoops := func(c clickerRuntime, stopCh <-chan struct{}) {
		rng := rand.New(rand.NewSource(time.Now().UnixNano()))
		cpsTicker := time.NewTicker(time.Second)
		stateTicker := time.NewTicker(150 * time.Millisecond)
		lastEnabled := c.IsEnabled()
		defer cpsTicker.Stop()
		defer stateTicker.Stop()

		applyCPS := func() {
			var min, max float64
			fyne.DoAndWait(func() {
				min = minSlider.Value
				max = maxSlider.Value
			})
			if max < min {
				min, max = max, min
			}
			cps := min
			if max > min {
				cps = min + rng.Float64()*(max-min)
			}
			if err := c.SetCPS(cps); err != nil {
				return
			}
			fyne.Do(func() {
				currentCPSText.SetText(fmt.Sprintf("Current CPS: %.2f", cps))
			})
		}

		applyCPS()
		for {
			select {
			case <-stopCh:
				return
			case <-cpsTicker.C:
				applyCPS()
			case <-stateTicker.C:
				enabled := c.IsEnabled()
				fyne.Do(func() {
					setEnabledStateUI(enabled)
					if enabled != lastEnabled {
						lastEnabled = enabled
						persistUISettings()
					}
				})
			}
		}
	}

	startRuntime := func(cfg config) error {
		logger := newSlogLogger(cfg.logLevel, appendLogLine)
		clicker, err := startClickerFromConfig(cfg, logger)
		if err != nil {
			return err
		}

		stop := make(chan struct{})
		stateMu.Lock()
		runningClicker = clicker
		runtimeStop = stop
		currentCfg = cfg
		stateMu.Unlock()

		go runRuntimeLoops(clicker, stop)

		fyne.Do(func() {
			errorText.Text = ""
			errorText.Refresh()
			setEnabledStateUI(clicker.IsEnabled())
			triggerCaptureBtn.SetText(displayCodeName(cfg.triggerRaw))
			toggleCaptureBtn.SetText(displayCodeName(cfg.toggleRaw))
		})
		return nil
	}

	runRuntimeTaskAsync := func(onDone func() error) {
		_, _, init := getState()
		if init {
			return
		}
		setInitializing(true)
		fyne.Do(func() {
			errorText.Text = ""
			errorText.Refresh()
			setInitializingUI(true)
		})

		go func() {
			err := onDone()
			fyne.Do(func() {
				setInitializing(false)
				setInitializingUI(false)
				if err != nil {
					switch {
					case isPermissionError(err):
						errorText.Text = permissionDeniedHint()
					case errors.Is(err, syscall.EBUSY) || strings.Contains(strings.ToLower(err.Error()), "device or resource busy"):
						errorText.Text = "Input device is in use by another app. Close the other app and try again."
					default:
						errorText.Text = err.Error()
					}
					errorText.Refresh()
					appendLogLine("ERROR " + errorText.Text)
					return
				}

				errorText.Text = ""
				errorText.Refresh()
				if clicker, _, _ := getState(); clicker != nil {
					setEnabledStateUI(clicker.IsEnabled())
				}
				persistUISettings()
			})
		}()
	}

	buildCfgFromUI := func() (config, error) {
		_, cfg, _ := getState()

		trigger := strings.TrimSpace(cfg.triggerRaw)
		toggle := strings.TrimSpace(cfg.toggleRaw)
		if trigger == "" {
			trigger = "BTN_LEFT"
		}
		if toggle == "" {
			toggle = "BTN_EXTRA"
		}

		triggerCode, err := parseTriggerCode(trigger)
		if err != nil {
			return cfg, err
		}
		toggleCode, err := parseTriggerCode(toggle)
		if err != nil {
			return cfg, err
		}
		if triggerCode == toggleCode {
			return cfg, fmt.Errorf("toggle must be different from trigger")
		}

		cfg.triggerRaw = trigger
		cfg.toggleRaw = toggle
		cfg.triggerCode = triggerCode
		cfg.toggleCode = toggleCode
		cfg.cps = minSlider.Value
		cfg.jitter = int(math.Round(jitterSlider.Value))
		return cfg, nil
	}

	persistUISettings = func() {
		clicker, cfg, _ := getState()
		enabled := startupEnabled
		if clicker != nil {
			enabled = clicker.IsEnabled()
		}

		settings := uiSettings{
			MinCPS:  minSlider.Value,
			MaxCPS:  maxSlider.Value,
			Jitter:  int(math.Round(jitterSlider.Value)),
			Trigger: strings.TrimSpace(cfg.triggerRaw),
			Toggle:  strings.TrimSpace(cfg.toggleRaw),
			Enabled: enabled,
		}

		if err := saveUISettings(settings); err != nil {
			errorText.Text = fmt.Sprintf("Failed to save settings: %v", err)
			errorText.Refresh()
		}
	}

	enableToggleBtn.OnTapped = func() {
		clicker, _, _ := getState()
		if clicker == nil {
			return
		}
		clicker.SetEnabled(!clicker.IsEnabled())
		setEnabledStateUI(clicker.IsEnabled())
		persistUISettings()
	}

	triggerCaptureBtn.OnTapped = func() {
		clicker, _, _ := getState()
		if clicker == nil {
			return
		}

		cfg, err := buildCfgFromUI()
		if err != nil {
			errorText.Text = err.Error()
			errorText.Refresh()
			appendLogLine("ERROR " + err.Error())
			return
		}

		appendLogLine("INFO Waiting for trigger input")
		runRuntimeTaskAsync(func() error {
			prevClicker, prevCfg, _ := getState()
			if prevClicker == nil {
				return fmt.Errorf("runtime is not initialized")
			}
			prevEnabled := prevClicker.IsEnabled()
			prevCfg.startEnabled = prevEnabled

			prevTriggerRaw := prevCfg.triggerRaw
			capturedFromRuntime := true
			code, err := prevClicker.CaptureNextKeyCode(2 * time.Second)
			if err != nil {
				capturedFromRuntime = false
				stopRuntime()
				code, err = captureNextCode(cfg.backend, "", 10*time.Second)
				if err != nil {
					_ = startRuntime(prevCfg)
					return err
				}
			}
			if code == cfg.toggleCode {
				if !capturedFromRuntime {
					_ = startRuntime(prevCfg)
				}
				return fmt.Errorf("captured trigger %s matches toggle; choose a different key/button", formatCodeName(code))
			}

			cfg.triggerCode = code
			cfg.triggerRaw = formatCodeName(code)
			cfg.startEnabled = prevEnabled

			fyne.DoAndWait(func() {
				triggerCaptureBtn.SetText(displayCodeName(cfg.triggerRaw))
			})

			if capturedFromRuntime {
				prevClicker.SetTriggerCode(code)
				setCurrentCfg(cfg)
				appendLogLine("INFO Captured trigger " + cfg.triggerRaw)
				return nil
			}

			stopRuntime()
			if err := startRuntime(cfg); err != nil {
				_ = startRuntime(prevCfg)
				fyne.DoAndWait(func() {
					triggerCaptureBtn.SetText(displayCodeName(prevTriggerRaw))
				})
				return err
			}

			appendLogLine("INFO Captured trigger " + cfg.triggerRaw)
			return nil
		})
	}

	toggleCaptureBtn.OnTapped = func() {
		clicker, _, _ := getState()
		if clicker == nil {
			return
		}

		cfg, err := buildCfgFromUI()
		if err != nil {
			errorText.Text = err.Error()
			errorText.Refresh()
			appendLogLine("ERROR " + err.Error())
			return
		}

		appendLogLine("INFO Waiting for toggle input")
		runRuntimeTaskAsync(func() error {
			prevClicker, prevCfg, _ := getState()
			if prevClicker == nil {
				return fmt.Errorf("runtime is not initialized")
			}
			prevEnabled := prevClicker.IsEnabled()
			prevCfg.startEnabled = prevEnabled

			prevToggleRaw := prevCfg.toggleRaw
			capturedFromRuntime := true
			code, err := prevClicker.CaptureNextKeyCode(2 * time.Second)
			if err != nil {
				capturedFromRuntime = false
				// Fallback to global capture when runtime source set does not see desired key/button.
				stopRuntime()
				code, err = captureNextCode(cfg.backend, "", 10*time.Second)
				if err != nil {
					_ = startRuntime(prevCfg)
					return err
				}
			}
			if code == cfg.triggerCode {
				if !capturedFromRuntime {
					_ = startRuntime(prevCfg)
				}
				return fmt.Errorf("captured toggle %s matches trigger; choose a different key/button", formatCodeName(code))
			}

			cfg.toggleCode = code
			cfg.toggleRaw = formatCodeName(code)
			cfg.startEnabled = prevEnabled

			fyne.DoAndWait(func() {
				toggleCaptureBtn.SetText(displayCodeName(cfg.toggleRaw))
			})

			// Fast path: capture from live runtime stream, update toggle in-place.
			if capturedFromRuntime {
				prevClicker.SetToggleCode(code)
				setCurrentCfg(cfg)
				appendLogLine("INFO Captured toggle " + cfg.toggleRaw)
				return nil
			}

			stopRuntime()
			if err := startRuntime(cfg); err != nil {
				_ = startRuntime(prevCfg)
				fyne.DoAndWait(func() {
					toggleCaptureBtn.SetText(displayCodeName(prevToggleRaw))
				})
				return err
			}

			appendLogLine("INFO Captured toggle " + cfg.toggleRaw)
			return nil
		})
	}

	startupCfg, err := buildCfgFromUI()
	if err != nil {
		return err
	}
	startupCfg.startEnabled = startupEnabled

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	var closeOnce sync.Once
	cleanup := func() {
		closeOnce.Do(func() {
			stopRuntime()
		})
	}

	requestQuit := func() {
		fyne.Do(func() {
			persistUISettings()
			cleanup()
			if currentApp := fyne.CurrentApp(); currentApp != nil {
				currentApp.Quit()
				return
			}
			window.SetCloseIntercept(nil)
			window.Close()
		})
	}

	go func() {
		<-sigCh
		requestQuit()
	}()

	// Some GUI backends can leave Ctrl+C as raw ETX byte instead of SIGINT.
	go func() {
		buf := make([]byte, 1)
		for {
			n, err := os.Stdin.Read(buf)
			if err != nil {
				return
			}
			if n == 1 && buf[0] == 3 {
				requestQuit()
				return
			}
		}
	}()

	window.SetCloseIntercept(func() {
		persistUISettings()
		cleanup()
		if currentApp := fyne.CurrentApp(); currentApp != nil {
			currentApp.Quit()
			return
		}
		window.SetCloseIntercept(nil)
		window.Close()
	})

	titleText := canvas.NewText("🗿 CLICKER", color.NRGBA{R: 0xff, G: 0x75, B: 0x75, A: 0xff})
	titleText.TextStyle = fyne.TextStyle{Bold: true}
	titleText.TextSize = 30

	accentLine := canvas.NewRectangle(color.NRGBA{R: 0xff, G: 0x66, B: 0x66, A: 0xff})
	accentLine.SetMinSize(fyne.NewSize(220, 3))

	newSliderControl := func(label string, value *widget.Label, slider *widget.Slider) fyne.CanvasObject {
		title := widget.NewLabel(label)
		title.TextStyle = fyne.TextStyle{Bold: true}
		head := container.NewBorder(nil, nil, title, value, nil)
		return container.NewVBox(head, slider)
	}

	rateControls := container.NewVBox(
		newSliderControl("Min CPS", minValue, minSlider),
		newSliderControl("Max CPS", maxValue, maxSlider),
		newSliderControl("Jitter", jitterValue, jitterSlider),
	)
	keybindControls := widget.NewForm(
		widget.NewFormItem("Trigger", triggerCaptureBtn),
		widget.NewFormItem("Toggle", toggleCaptureBtn),
	)
	rateCard := widget.NewCard("Rate", "", rateControls)
	keybindCard := widget.NewCard("Keybinds", "", keybindControls)
	controlsRow := container.NewGridWithColumns(2, rateCard, keybindCard)

	mainContent := container.NewVBox(
		titleText,
		accentLine,
		controlsRow,
		currentCPSText,
		errorText,
		initProgress,
		enableToggleBtn,
	)
	mainPanel := container.NewPadded(mainContent)

	var rootContent fyne.CanvasObject = mainPanel
	if debugLogs {
		logsCard := widget.NewCard("Logs", "", logScroll)
		split := container.NewVSplit(mainPanel, logsCard)
		split.SetOffset(0.68)
		rootContent = split
	}

	setInitializingUI(true)
	appendLogLine("INFO Initializing input devices...")
	runRuntimeTaskAsync(func() error {
		if err := startRuntime(startupCfg); err != nil {
			return err
		}
		appendLogLine("INFO Initialization complete")
		return nil
	})

	window.SetContent(rootContent)
	window.ShowAndRun()
	cleanup()
	return nil
}
