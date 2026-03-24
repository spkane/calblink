package main

import (
	"errors"
	"sync"
	"testing"
	"time"
)

type fakeBlinkDevice struct {
	mu           sync.Mutex
	states       []LightState
	failuresLeft int
	panicNextSet bool
	panicOnClose bool
	closeCount   int
}

func newFakeBlinkDevice() *fakeBlinkDevice {
	return &fakeBlinkDevice{}
}

func (d *fakeBlinkDevice) Close() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.closeCount++
	if d.panicOnClose {
		d.panicOnClose = false
		panic("close panic")
	}
}

func (d *fakeBlinkDevice) SetState(state LightState) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.panicNextSet {
		d.panicNextSet = false
		panic("set panic")
	}
	d.states = append(d.states, state)
	if d.failuresLeft > 0 {
		d.failuresLeft--
		return errors.New("device disconnected")
	}
	return nil
}

func (d *fakeBlinkDevice) stateCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.states)
}

func (d *fakeBlinkDevice) closeCalls() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.closeCount
}

func withBlinkerTestHooks(t *testing.T, opener func() (blinkDevice, error), refreshInterval time.Duration) {
	t.Helper()
	previousOpen := openBlinkDeviceFn
	previousRefresh := steadyStateRefreshInterval
	openBlinkDeviceFn = opener
	steadyStateRefreshInterval = refreshInterval
	t.Cleanup(func() {
		openBlinkDeviceFn = previousOpen
		steadyStateRefreshInterval = previousRefresh
	})
}

func newInitializedBlinker(t *testing.T, maxFailures int) *BlinkerState {
	t.Helper()
	blinker := &BlinkerState{
		newState:    make(chan CalendarState, 1),
		maxFailures: maxFailures,
		done:        make(chan struct{}),
	}
	if err := blinker.reinitialize(); err != nil {
		t.Fatalf("reinitialize() failed: %v", err)
	}
	return blinker
}

func waitForCondition(t *testing.T, timeout time.Duration, condition func() bool, message string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for condition: %s", message)
}

func TestPatternRunnerRefreshesSteadyState(t *testing.T) {
	device := newFakeBlinkDevice()
	withBlinkerTestHooks(t, func() (blinkDevice, error) {
		return device, nil
	}, 20*time.Millisecond)

	blinker := newInitializedBlinker(t, 3)
	go blinker.patternRunner()
	t.Cleanup(blinker.shutdown)

	Green.Execute(blinker)
	waitForCondition(t, 250*time.Millisecond, func() bool {
		return device.stateCount() >= 2
	}, "initial steady-state writes")

	initialCount := device.stateCount()
	waitForCondition(t, 250*time.Millisecond, func() bool {
		return device.stateCount() >= initialCount+2
	}, "steady-state refresh writes")
}

func TestPatternRunnerRecoversOnSteadyStateRefresh(t *testing.T) {
	firstDevice := newFakeBlinkDevice()
	secondDevice := newFakeBlinkDevice()
	var openMu sync.Mutex
	openCount := 0

	withBlinkerTestHooks(t, func() (blinkDevice, error) {
		openMu.Lock()
		defer openMu.Unlock()
		openCount++
		if openCount == 1 {
			return firstDevice, nil
		}
		return secondDevice, nil
	}, 20*time.Millisecond)

	blinker := newInitializedBlinker(t, 3)
	go blinker.patternRunner()
	t.Cleanup(blinker.shutdown)

	Green.Execute(blinker)
	waitForCondition(t, 250*time.Millisecond, func() bool {
		return firstDevice.stateCount() >= 2
	}, "initial steady-state writes")

	firstDevice.mu.Lock()
	firstDevice.failuresLeft = 1
	firstDevice.mu.Unlock()

	waitForCondition(t, 500*time.Millisecond, func() bool {
		openMu.Lock()
		defer openMu.Unlock()
		return openCount >= 2
	}, "device reopen after refresh failure")
	waitForCondition(t, 500*time.Millisecond, func() bool {
		return secondDevice.stateCount() >= 2
	}, "steady state applied to reopened device")
}

func TestSetStateRecoversFromPanickingDevice(t *testing.T) {
	firstDevice := newFakeBlinkDevice()
	firstDevice.panicNextSet = true
	secondDevice := newFakeBlinkDevice()
	var openMu sync.Mutex
	openCount := 0

	withBlinkerTestHooks(t, func() (blinkDevice, error) {
		openMu.Lock()
		defer openMu.Unlock()
		openCount++
		if openCount == 1 {
			return firstDevice, nil
		}
		return secondDevice, nil
	}, 20*time.Millisecond)

	blinker := newInitializedBlinker(t, 3)
	t.Cleanup(blinker.shutdown)

	err := blinker.setState(LightState{Red: 255, LED: LED1})
	if err != nil {
		t.Fatalf("setState() returned error after panic recovery: %v", err)
	}

	waitForCondition(t, 250*time.Millisecond, func() bool {
		return secondDevice.stateCount() >= 1
	}, "replacement device write after panic")
	if got := firstDevice.closeCalls(); got == 0 {
		t.Fatalf("expected panicking device to be closed, got %d close calls", got)
	}
}
