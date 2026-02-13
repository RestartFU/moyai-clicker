//go:build windows

package main

import (
	"fmt"
	"log/slog"
	"math"
	"strings"
	"time"

	"clicker/internal/adapters/wininput"
)

func parseTriggerCode(value string) (uint16, error) {
	return wininput.ParseCode(value)
}

func parseBackendChoice(value string) (string, error) {
	backend := strings.ToLower(strings.TrimSpace(value))
	if backend == "" {
		backend = "auto"
	}
	switch backend {
	case "auto", "windows":
		return backend, nil
	default:
		return "", fmt.Errorf("invalid --backend %q (windows supports auto|windows)", value)
	}
}

func captureNextCode(_ string, _ string, timeout time.Duration) (uint16, error) {
	return wininput.CaptureNextKeyCode(timeout)
}

func formatCodeName(code uint16) string {
	return wininput.FormatCodeName(code)
}

func listInputDevices(_ string) error {
	devices, err := wininput.ListInputDevices()
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

func defaultGrabForTrigger(_ uint16) bool {
	return false
}

func permissionDeniedHint() string {
	return "Permission denied registering global input hooks. Run as Administrator and ensure input-hooking is allowed."
}

func startClickerFromConfig(cfg config, logger *slog.Logger) (clickerRuntime, error) {
	if cfg.devicePath != "" {
		logger.Warn("--device is ignored on Windows; using global keyboard/mouse hooks")
	}
	if cfg.grabDevices {
		logger.Warn("--grab is not supported on Windows and will be ignored")
	}

	clickDown := time.Duration(math.Max(0, cfg.downMS) * float64(time.Millisecond))
	runtime, err := wininput.NewRuntime(
		wininput.RuntimeConfig{
			TriggerCode:  cfg.triggerCode,
			ToggleCode:   cfg.toggleCode,
			CPS:          cfg.cps,
			ClickDown:    clickDown,
			StartEnabled: cfg.startEnabled,
		},
		logger,
	)
	if err != nil {
		return nil, err
	}

	if err := runtime.Start(); err != nil {
		runtime.Stop()
		return nil, err
	}

	logger.Info("Trigger", "name", formatCodeName(cfg.triggerCode), "code", cfg.triggerCode)
	logger.Info("Toggle", "name", formatCodeName(cfg.toggleCode), "code", cfg.toggleCode)
	logger.Info("Rate", "cps", cfg.cps)
	logger.Info("Input mode", "mode", "windows-global-hooks")
	if cfg.startEnabled {
		logger.Info("Initial state enabled (press toggle to disable/enable)")
	} else {
		logger.Info("Initial state disabled (press toggle to enable/disable)")
	}
	logger.Info("Hold trigger to autoclick left mouse button. Press Ctrl+C to stop")
	return runtime, nil
}
