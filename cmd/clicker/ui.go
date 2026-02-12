package main

import (
	"fmt"
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
	Stop()
}

func runUI(baseCfg config) error {
	fApp := app.New()
	fApp.Settings().SetTheme(theme.DarkTheme())

	window := fApp.NewWindow("Auto-Clicker")
	window.Resize(fyne.NewSize(760, 620))
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

	minSlider := widget.NewSlider(1, 30)
	minSlider.Step = 1
	minSlider.SetValue(minDefault)

	maxSlider := widget.NewSlider(1, 30)
	maxSlider.Step = 1
	maxSlider.SetValue(maxDefault)

	minText := canvas.NewText("", nil)
	maxText := canvas.NewText("", nil)
	updateMinMaxText := func() {
		minText.Text = fmt.Sprintf("Min CPS: %.0f", minSlider.Value)
		minText.Refresh()
		maxText.Text = fmt.Sprintf("Max CPS: %.0f", maxSlider.Value)
		maxText.Refresh()
	}
	updateMinMaxText()

	persistUISettings := func() {}

	minSlider.OnChanged = func(v float64) {
		if v > maxSlider.Value {
			maxSlider.SetValue(v)
		}
		updateMinMaxText()
		persistUISettings()
	}
	maxSlider.OnChanged = func(v float64) {
		if v < minSlider.Value {
			minSlider.SetValue(v)
		}
		updateMinMaxText()
		persistUISettings()
	}

	triggerEntry := widget.NewEntry()
	triggerEntry.SetText(triggerRaw)
	triggerEntry.SetPlaceHolder("BTN_LEFT")

	toggleEntry := widget.NewEntry()
	toggleEntry.SetText(toggleRaw)
	toggleEntry.SetPlaceHolder("BTN_EXTRA")

	keybindInfo := canvas.NewText("", nil)
	setKeybindInfo := func(trigger, toggle string) {
		keybindInfo.Text = fmt.Sprintf(
			"Current Keybinds: trigger=%s, toggle=%s",
			strings.ToUpper(strings.TrimSpace(trigger)),
			strings.ToUpper(strings.TrimSpace(toggle)),
		)
		keybindInfo.Refresh()
	}
	setKeybindInfo(triggerRaw, toggleRaw)

	statusText := canvas.NewText("Status: starting...", nil)
	errorText := canvas.NewText("", nil)
	errorText.Color = theme.Color(theme.ColorNameError)
	if settingsLoadWarning != "" {
		errorText.Text = settingsLoadWarning
	}
	currentCPSText := canvas.NewText("Current CPS: -", nil)
	logGrid := widget.NewTextGrid()
	logGrid.SetText("")
	logScroll := container.NewVScroll(logGrid)
	logScroll.SetMinSize(fyne.NewSize(0, 150))

	const maxUILogLines = 50
	var logMu sync.Mutex
	logLines := make([]string, 0, maxUILogLines)
	appendLogLine := func(line string) {
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

	startBtn := widget.NewButton("Start", nil)
	stopBtn := widget.NewButton("Stop", nil)
	applyKeybindBtn := widget.NewButton("Apply Keybinds", nil)
	initProgress := widget.NewProgressBarInfinite()
	initProgress.Hide()

	setEnabledStateUI := func(enabled bool) {
		if enabled {
			statusText.Text = "Status: enabled"
			startBtn.Disable()
			stopBtn.Enable()
		} else {
			statusText.Text = "Status: paused"
			startBtn.Enable()
			stopBtn.Disable()
		}
		statusText.Refresh()
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

	setInitializingUI := func(v bool, text string) {
		if v {
			startBtn.Disable()
			stopBtn.Disable()
			applyKeybindBtn.Disable()
			initProgress.Show()
			statusText.Text = text
			statusText.Refresh()
			return
		}
		applyKeybindBtn.Enable()
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
				currentCPSText.Text = fmt.Sprintf("Current CPS: %.2f", cps)
				currentCPSText.Refresh()
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
			setKeybindInfo(cfg.triggerRaw, cfg.toggleRaw)
		})
		return nil
	}

	restartRuntime := func(cfg config) error {
		prevClicker, prevCfg, _ := getState()
		prevEnabled := true
		if prevClicker != nil {
			prevEnabled = prevClicker.IsEnabled()
		}
		cfg.startEnabled = prevEnabled
		prevCfg.startEnabled = prevEnabled

		stopRuntime()
		if err := startRuntime(cfg); err != nil {
			_ = startRuntime(prevCfg)
			return err
		}
		return nil
	}

	runRuntimeTaskAsync := func(initStatus string, onDone func() error) {
		_, _, init := getState()
		if init {
			return
		}
		setInitializing(true)
		fyne.Do(func() {
			errorText.Text = ""
			errorText.Refresh()
			setInitializingUI(true, initStatus)
		})

		go func() {
			err := onDone()
			fyne.Do(func() {
				setInitializing(false)
				setInitializingUI(false, "")
				if err != nil {
					if isPermissionError(err) {
						errorText.Text = "permission denied for /dev/input or /dev/uinput (run as root or set udev rules)"
					} else {
						errorText.Text = err.Error()
					}
					errorText.Refresh()
					appendLogLine("ERROR " + errorText.Text)

					clicker, _, _ := getState()
					if clicker == nil {
						startBtn.Disable()
						stopBtn.Disable()
						statusText.Text = "Status: initialization failed"
						statusText.Refresh()
					}
					return
				}

				clicker, _, _ := getState()
				if clicker == nil {
					startBtn.Disable()
					stopBtn.Disable()
					statusText.Text = "Status: not initialized"
					statusText.Refresh()
					return
				}
				setEnabledStateUI(clicker.IsEnabled())
				persistUISettings()
			})
		}()
	}

	buildCfgFromUI := func() (config, error) {
		cfg := currentCfg

		trigger := strings.TrimSpace(triggerEntry.Text)
		toggle := strings.TrimSpace(toggleEntry.Text)
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
		return cfg, nil
	}

	persistUISettings = func() {
		clicker, _, _ := getState()
		enabled := startupEnabled
		if clicker != nil {
			enabled = clicker.IsEnabled()
		}

		settings := uiSettings{
			MinCPS:  minSlider.Value,
			MaxCPS:  maxSlider.Value,
			Trigger: strings.TrimSpace(triggerEntry.Text),
			Toggle:  strings.TrimSpace(toggleEntry.Text),
			Enabled: enabled,
		}

		if err := saveUISettings(settings); err != nil {
			errorText.Text = fmt.Sprintf("Failed to save settings: %v", err)
			errorText.Refresh()
		}
	}

	startBtn.OnTapped = func() {
		clicker, _, init := getState()
		if init {
			return
		}
		if clicker == nil {
			return
		}
		clicker.SetEnabled(true)
		setEnabledStateUI(true)
		persistUISettings()
		appendLogLine("INFO Enabled from UI")
	}

	stopBtn.OnTapped = func() {
		clicker, _, init := getState()
		if init {
			return
		}
		if clicker == nil {
			return
		}
		clicker.SetEnabled(false)
		setEnabledStateUI(false)
		persistUISettings()
		appendLogLine("INFO Paused from UI")
	}

	applyKeybindBtn.OnTapped = func() {
		cfg, err := buildCfgFromUI()
		if err != nil {
			errorText.Text = err.Error()
			errorText.Refresh()
			appendLogLine("ERROR " + err.Error())
			return
		}

		appendLogLine("INFO Applying keybind changes")
		runRuntimeTaskAsync("Status: applying keybinds...", func() error {
			if err := restartRuntime(cfg); err != nil {
				return err
			}
			appendLogLine("INFO Keybind changes applied")
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

	content := container.NewVBox(
		minText,
		minSlider,
		maxText,
		maxSlider,
		widget.NewSeparator(),
		widget.NewLabel("Keybinds"),
		widget.NewForm(
			widget.NewFormItem("Trigger", triggerEntry),
			widget.NewFormItem("Toggle", toggleEntry),
		),
		applyKeybindBtn,
		keybindInfo,
		widget.NewSeparator(),
		container.NewGridWithColumns(2, startBtn, stopBtn),
		currentCPSText,
		statusText,
		errorText,
	)

	controlsCard := widget.NewCard("Controls", "", content)
	logsCard := widget.NewCard("Logs", "", logScroll)
	split := container.NewVSplit(controlsCard, logsCard)
	split.SetOffset(0.45)

	setInitializingUI(true, "Status: initializing...")
	appendLogLine("INFO Initializing input devices...")
	runRuntimeTaskAsync("Status: initializing...", func() error {
		if err := startRuntime(startupCfg); err != nil {
			return err
		}
		appendLogLine("INFO Initialization complete")
		return nil
	})

	window.SetContent(split)
	window.ShowAndRun()
	cleanup()
	return nil
}
