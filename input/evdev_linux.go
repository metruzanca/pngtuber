//go:build linux

package input

import (
	"errors"
	"github.com/charmbracelet/log"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"

	"github.com/grafov/evdev"
)

// signalBufferSize bounds the backend channel. The producer drops signals
// when it is full rather than stalling the device read loop.
const signalBufferSize = 256

// evdevBackend polls keyboard/mouse devices under /dev/input/event* and emits
// coarse activity events on a bounded channel.
type evdevBackend struct {
	signals   chan Event
	devices   []*evdev.InputDevice
	closed    atomic.Bool
	closeOnce sync.Once
}

// NewBackend returns the Linux global-input backend: it scans
// /dev/input/event* for keyboard/pointer devices and starts a poll goroutine
// per device. If the devices can't be opened (e.g. the user is not in the
// input group) it logs a clear, actionable warning and returns a no-op backend
// so the app keeps running (mic-only mode) instead of crashing.
func NewBackend() Backend {
	paths, err := filepath.Glob("/dev/input/event*")
	if err != nil {
		// Glob only fails on a malformed pattern; treat it as no devices.
		paths = nil
	}

	b := &evdevBackend{signals: make(chan Event, signalBufferSize)}
	sawPermissionError := false

	for _, p := range paths {
		dev, err := evdev.Open(p)
		if err != nil {
			if errors.Is(err, os.ErrPermission) {
				sawPermissionError = true
			}
			log.Warnf("input: skip %s: %v", p, err)
			continue
		}
		if !deviceRelevant(dev) {
			dev.File.Close()
			continue
		}
		b.devices = append(b.devices, dev)
	}

	if len(b.devices) == 0 {
		switch {
		case len(paths) == 0:
			log.Warnf("input: no /dev/input/event* devices found; input monitoring disabled (running mic-only)")
		case sawPermissionError:
			log.Warnf("input: cannot read /dev/input/event* (permission denied). Add your user to the input group and re-login, then restart: sudo usermod -aG input $USER")
		default:
			log.Warnf("input: no keyboard/mouse devices found; input monitoring disabled (running mic-only)")
		}
		return NewNone()
	}

	for _, dev := range b.devices {
		go b.poll(dev)
	}
	log.Infof("input: monitoring %d device(s)", len(b.devices))
	return b
}

// deviceRelevant reports whether a device can produce keyboard or pointer
// activity we care about: any EV_KEY (keys and buttons), relative axes
// (EV_REL), or absolute pointer axes (EV_ABS).
func deviceRelevant(dev *evdev.InputDevice) bool {
	return len(dev.Capabilities["EV_KEY"]) > 0 ||
		len(dev.Capabilities["EV_REL"]) > 0 ||
		len(dev.Capabilities["EV_ABS"]) > 0
}

// poll reads a device forever, classifying events into signals. It returns
// when the device is closed (backend shutdown) or disappears.
func (b *evdevBackend) poll(dev *evdev.InputDevice) {
	name := dev.Name
	if name == "" {
		name = dev.Fn
	}
	for {
		events, err := dev.Read()
		if err != nil {
			if b.closed.Load() {
				return
			}
			if errors.Is(err, syscall.ENODEV) || errors.Is(err, syscall.EBADF) ||
				errors.Is(err, fs.ErrClosed) || errors.Is(err, os.ErrClosed) {
				log.Warnf("input: device %s gone (%v); stopped monitoring it", name, err)
				return
			}
			// Transient errors (EINTR etc.); keep polling.
			continue
		}
		for _, e := range events {
			ev, ok := eventFrom(e)
			if !ok {
				continue
			}
			b.emit(ev)
		}
	}
}

// eventFrom converts one evdev event into an input Event, carrying relative
// pointer motion for EV_REL motion events.
func eventFrom(e evdev.InputEvent) (Event, bool) {
	sig, ok := classify(e)
	if !ok {
		return Event{}, false
	}
	ev := Event{Signal: sig}
	if e.Type == evdev.EV_REL {
		switch e.Code {
		case evdev.REL_X:
			ev.DX = e.Value
		case evdev.REL_Y:
			ev.DY = e.Value
		}
	}
	return ev, true
}

// emit pushes an event into the bounded channel, dropping it when full.
func (b *evdevBackend) emit(ev Event) {
	select {
	case b.signals <- ev:
	default:
	}
}

func (b *evdevBackend) Events() <-chan Event { return b.signals }

func (b *evdevBackend) Close() {
	b.closeOnce.Do(func() {
		b.closed.Store(true)
		for _, dev := range b.devices {
			// Closing the fd unblocks Read(); poll() then sees the closed
			// flag and exits. The channel is intentionally left open so a
			// racing emit can't panic on a send to a closed channel.
			dev.File.Close()
		}
	})
}

// classify maps a single evdev event to a coarse Signal. ok is false for
// events that shouldn't count as activity (key releases, auto-repeat, sync).
func classify(e evdev.InputEvent) (Signal, bool) {
	switch e.Type {
	case evdev.EV_KEY:
		// Value semantics: 0 release, 1 press, 2 auto-repeat. Count presses.
		if e.Value != 1 {
			return 0, false
		}
		if isMouseButton(int(e.Code)) {
			return MouseActivity, true
		}
		return KeyActivity, true
	case evdev.EV_REL, evdev.EV_ABS:
		return MouseActivity, true
	default:
		return 0, false
	}
}

// isMouseButton reports whether an EV_KEY code is a button (BTN_*) rather
// than a keyboard key, using the kernel's BTN code table.
func isMouseButton(code int) bool {
	return evdev.BTN[code] != ""
}
