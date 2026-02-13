package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"math"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"clicker/internal/adapters/linuxinput"
)

type config struct {
	triggerCode  uint16
	toggleCode   uint16
	triggerRaw   string
	toggleRaw    string
	devicePath   string
	cps          float64
	downMS       float64
	startEnabled bool
	listDevices  bool
	grabDevices  bool
	ui           bool
	logLevel     slog.Level
}

type lineSinkWriter struct {
	sink  func(line string)
	mu    sync.Mutex
	lines bytes.Buffer
}

func (w *lineSinkWriter) Write(p []byte) (int, error) {
	if w.sink == nil {
		return len(p), nil
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	total := len(p)
	for len(p) > 0 {
		idx := bytes.IndexByte(p, '\n')
		if idx == -1 {
			_, _ = w.lines.Write(p)
			break
		}
		_, _ = w.lines.Write(p[:idx])
		line := strings.TrimSpace(w.lines.String())
		w.lines.Reset()
		if line != "" {
			w.sink(line)
		}
		p = p[idx+1:]
	}
	return total, nil
}

func newSlogLogger(level slog.Level, sink func(line string)) *slog.Logger {
	if !debugLogsEnabled() {
		return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{
			Level: level,
		}))
	}

	out := io.Writer(os.Stderr)
	if sink != nil {
		out = io.MultiWriter(os.Stderr, &lineSinkWriter{sink: sink})
	}

	return slog.New(slog.NewTextHandler(out, &slog.HandlerOptions{
		Level: level,
	}))
}

func debugLogsEnabled() bool {
	return strings.TrimSpace(os.Getenv("DEBUG")) == "1"
}

func parseLogLevel(value string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warning", "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("invalid --log-level %q (expected debug|info|warning|error)", value)
	}
}

func parseTriggerCode(value string) (uint16, error) {
	return linuxinput.ParseCode(value)
}

func captureNextCode(devicePath string, timeout time.Duration) (uint16, error) {
	return linuxinput.CaptureNextKeyCode(devicePath, timeout)
}

func formatCodeName(code uint16) string {
	return linuxinput.FormatCodeName(code)
}

func listInputDevices() error {
	devices, err := linuxinput.ListInputDevices()
	if err != nil {
		return err
	}
	for _, dev := range devices {
		virtualTag := "physical"
		if dev.IsVirtual {
			virtualTag = "virtual"
		}
		pointerTag := "non-pointer"
		if dev.IsPointer {
			pointerTag = "pointer"
		}
		fmt.Printf("%s: %s [%s, %s]\n", dev.Path, dev.Name, virtualTag, pointerTag)
	}
	return nil
}

func parseConfig(args []string) (config, error) {
	cfg := config{startEnabled: true}
	flags := flag.NewFlagSet("clicker", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)

	var triggerRaw string
	var toggleRaw string
	var logLevelRaw string
	var noGrab bool
	var cliMode bool

	flags.StringVar(&triggerRaw, "trigger", "BTN_LEFT", "Trigger key/button code name (default: BTN_LEFT). Example: BTN_SIDE, KEY_LEFTALT.")
	flags.StringVar(&toggleRaw, "toggle", "BTN_EXTRA", "Enable/disable autoclicker when pressed (default: BTN_EXTRA, usually mouse button 5).")
	flags.StringVar(&cfg.devicePath, "device", "", "Input event device path to listen on, e.g. /dev/input/event4. Auto-detected if omitted.")
	flags.Float64Var(&cfg.cps, "cps", 16.0, "Clicks per second while held.")
	flags.Float64Var(&cfg.downMS, "down-ms", 10.0, "How long each synthetic click stays down in ms (default: 10).")
	flags.BoolVar(&cfg.listDevices, "list-devices", false, "Print available input devices and exit.")
	flags.BoolVar(&cfg.grabDevices, "grab", false, "Grab source devices and suppress raw trigger events (recommended for BTN_LEFT on Wayland).")
	flags.BoolVar(&noGrab, "no-grab", false, "Disable source device grabbing.")
	flags.BoolVar(&cfg.ui, "ui", true, "Start desktop GUI (Fyne) by default. Use --ui=false or --cli for terminal mode.")
	flags.BoolVar(&cliMode, "cli", false, "Force terminal mode (disables GUI).")
	flags.StringVar(&logLevelRaw, "log-level", "info", "Log verbosity (default: info). Allowed: debug, info, warning, error.")

	if err := flags.Parse(args); err != nil {
		return cfg, err
	}
	if flags.NArg() > 0 {
		return cfg, fmt.Errorf("unexpected arguments: %s", strings.Join(flags.Args(), " "))
	}
	if cfg.cps <= 0 {
		return cfg, fmt.Errorf("--cps must be > 0")
	}
	if cfg.grabDevices && noGrab {
		return cfg, fmt.Errorf("--grab and --no-grab are mutually exclusive")
	}
	if cliMode {
		cfg.ui = false
	}

	triggerCode, err := parseTriggerCode(triggerRaw)
	if err != nil {
		return cfg, err
	}
	toggleCode, err := parseTriggerCode(toggleRaw)
	if err != nil {
		return cfg, err
	}
	if triggerCode == toggleCode {
		return cfg, fmt.Errorf("--toggle must be different from --trigger")
	}

	if !cfg.grabDevices && !noGrab {
		cfg.grabDevices = triggerCode == linuxinput.CodeBTNLeft
	}

	parsedLevel, err := parseLogLevel(logLevelRaw)
	if err != nil {
		return cfg, err
	}

	cfg.triggerCode = triggerCode
	cfg.toggleCode = toggleCode
	cfg.triggerRaw = triggerRaw
	cfg.toggleRaw = toggleRaw
	cfg.logLevel = parsedLevel
	return cfg, nil
}

func isPermissionError(err error) bool {
	return errors.Is(err, os.ErrPermission) || errors.Is(err, syscall.EPERM) || errors.Is(err, syscall.EACCES)
}

func startClickerFromConfig(cfg config, logger *slog.Logger) (*linuxinput.Runtime, error) {
	selection, err := linuxinput.OpenSourceSelection(cfg.devicePath, cfg.triggerCode, cfg.toggleCode)
	if err != nil {
		return nil, err
	}

	for _, dev := range selection.Devices {
		name, _ := dev.Name()
		logger.Info("Using source device", "path", dev.Path(), "name", name)
	}

	clickDown := time.Duration(math.Max(0, cfg.downMS) * float64(time.Millisecond))
	runtime, err := linuxinput.NewRuntime(
		selection,
		linuxinput.RuntimeConfig{
			TriggerCode:  cfg.triggerCode,
			ToggleCode:   cfg.toggleCode,
			CPS:          cfg.cps,
			ClickDown:    clickDown,
			StartEnabled: cfg.startEnabled,
			GrabDevices:  cfg.grabDevices,
		},
		logger,
	)
	if err != nil {
		for _, dev := range selection.Devices {
			_ = dev.Close()
		}
		return nil, err
	}

	if err := runtime.Start(); err != nil {
		runtime.Stop()
		return nil, err
	}

	logger.Info("Trigger", "name", formatCodeName(cfg.triggerCode), "code", cfg.triggerCode)
	logger.Info("Toggle", "name", formatCodeName(cfg.toggleCode), "code", cfg.toggleCode)
	logger.Info("Rate", "cps", cfg.cps)
	if runtime.GrabEnabled() {
		logger.Info("Grab mode enabled")
	} else {
		logger.Info("Grab mode disabled")
	}
	if cfg.triggerCode == linuxinput.CodeBTNLeft && !runtime.GrabEnabled() {
		logger.Warn("BTN_LEFT trigger without grabbing may be ignored by Wayland; use --grab")
	}
	if cfg.startEnabled {
		logger.Info("Initial state enabled (press toggle to disable/enable)")
	} else {
		logger.Info("Initial state disabled (press toggle to enable/disable)")
	}
	logger.Info("Hold trigger to autoclick left mouse button. Press Ctrl+C to stop")
	return runtime, nil
}

func run(args []string, stderr io.Writer) int {
	cfg, err := parseConfig(args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		fmt.Fprintln(stderr, err)
		return 2
	}

	if cfg.listDevices {
		if err := listInputDevices(); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	}

	if cfg.ui {
		if err := runUI(cfg); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	}

	logger := newSlogLogger(cfg.logLevel, nil)
	runtime, err := startClickerFromConfig(cfg, logger)
	if err != nil {
		if isPermissionError(err) {
			fmt.Fprintln(stderr, "Permission denied opening input/uinput. Run as root or set udev rules for /dev/input and /dev/uinput.")
			return 1
		}
		fmt.Fprintln(stderr, err)
		return 1
	}
	defer runtime.Stop()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	<-ctx.Done()
	return 0
}

func main() {
	os.Exit(run(os.Args[1:], os.Stderr))
}
