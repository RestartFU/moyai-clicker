//go:build linux

package linuxinput

import (
	"fmt"
	"sort"
	"time"

	evdev "github.com/holoplot/go-evdev"
)

// CaptureNextKeyCode waits for the next pressed key/button (EV_KEY with value 1).
// If devicePath is empty, it listens on all non-virtual input devices with key capabilities.
func CaptureNextKeyCode(devicePath string, timeout time.Duration) (uint16, error) {
	devices, err := openCaptureDevices(devicePath)
	if err != nil {
		return 0, err
	}
	return captureNextFromDevices(devices, timeout)
}

func captureNextFromDevices(devices []*evdev.InputDevice, timeout time.Duration) (uint16, error) {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	defer closeInputDevices(devices)

	done := make(chan struct{})
	codeCh := make(chan uint16, 1)
	for _, dev := range devices {
		go captureDeviceLoop(dev, done, codeCh)
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case code := <-codeCh:
		close(done)
		return code, nil
	case <-timer.C:
		close(done)
		return 0, fmt.Errorf("timed out waiting for key/button input")
	}
}

func captureDeviceLoop(dev *evdev.InputDevice, done <-chan struct{}, codeCh chan<- uint16) {
	for {
		select {
		case <-done:
			return
		default:
		}

		event, err := dev.ReadOne()
		if err != nil {
			if isWouldBlockError(err) {
				if !sleepCapture(done, 10*time.Millisecond) {
					return
				}
				continue
			}
			if isDeviceClosedError(err) {
				return
			}
			if !sleepCapture(done, 25*time.Millisecond) {
				return
			}
			continue
		}
		if event == nil {
			continue
		}
		if event.Type == evdev.EV_KEY && event.Value == 1 {
			select {
			case codeCh <- uint16(event.Code):
			default:
			}
			return
		}
	}
}

func sleepCapture(done <-chan struct{}, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-done:
		return false
	case <-timer.C:
		return true
	}
}

func openCaptureDevices(devicePath string) ([]*evdev.InputDevice, error) {
	if devicePath != "" {
		dev, err := openInputDevice(devicePath)
		if err != nil {
			return nil, err
		}
		if len(dev.CapableEvents(evdev.EV_KEY)) == 0 {
			_ = dev.Close()
			return nil, fmt.Errorf("%s does not expose key/button events", devicePath)
		}
		if err := dev.NonBlock(); err != nil {
			_ = dev.Close()
			return nil, fmt.Errorf("failed to set nonblocking mode for %s: %w", dev.Path(), err)
		}
		return []*evdev.InputDevice{dev}, nil
	}

	paths, err := evdev.ListDevicePaths()
	if err != nil {
		return nil, err
	}
	sort.Slice(paths, func(i, j int) bool {
		return paths[i].Path < paths[j].Path
	})

	devices := make([]*evdev.InputDevice, 0, len(paths))
	for _, path := range paths {
		dev, err := openInputDevice(path.Path)
		if err != nil {
			continue
		}

		name := path.Name
		if actualName, nameErr := dev.Name(); nameErr == nil && actualName != "" {
			name = actualName
		}
		if deviceIsVirtual(dev, name) || len(dev.CapableEvents(evdev.EV_KEY)) == 0 {
			_ = dev.Close()
			continue
		}
		if err := dev.NonBlock(); err != nil {
			_ = dev.Close()
			continue
		}
		devices = append(devices, dev)
	}

	if len(devices) == 0 {
		return nil, fmt.Errorf("no readable input devices with key/button events found")
	}
	return devices, nil
}

func closeInputDevices(devices []*evdev.InputDevice) {
	for _, dev := range devices {
		_ = dev.Close()
	}
}
