// Copyright 2024 Google Inc.
// Modifications Copyright 2026 calblink contributors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// This file manages the blink(1) state.

package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

const failureRetries = 3

const startupSignalStep = 120 * time.Millisecond

// calendarState is a display state for the calendar event. It encapsulates both the colors to display and the flash duration.
type CalendarState struct {
	Name           string
	primary        LightState
	secondary      LightState
	primaryFlash   time.Duration
	secondaryFlash time.Duration
	alternate      bool
}

type LightState struct {
	Red      uint8
	Green    uint8
	Blue     uint8
	LED      uint8
	FadeTime time.Duration
	Duration time.Duration
}

type blinkDevice interface {
	Close()
	SetState(LightState) error
}

const (
	LEDAll uint8 = iota
	LED1
	LED2
)

var offLightState = LightState{Duration: 10 * time.Millisecond}

func (state CalendarState) Execute(blinker *BlinkerState) {
	select {
	case blinker.newState <- state:
	default:
		select {
		case <-blinker.newState:
		default:
		}
		blinker.newState <- state
	}
}

var (
	Black        = CalendarState{Name: "Black", primary: offLightState}
	Green        = CalendarState{Name: "Green", primary: LightState{Green: 255}, secondary: LightState{Green: 255}}
	Yellow       = CalendarState{Name: "Yellow", primary: LightState{Red: 255, Green: 160}, secondary: LightState{Red: 255, Green: 160}}
	Red          = CalendarState{Name: "Red", primary: LightState{Red: 255}, secondary: LightState{Red: 255}}
	RedFlash     = CalendarState{Name: "Red Flash", primary: LightState{Red: 255}, secondary: offLightState, primaryFlash: 500 * time.Millisecond, alternate: true}
	FastRedFlash = CalendarState{Name: "Fast Red Flash", primary: LightState{Red: 255}, secondary: offLightState, primaryFlash: 125 * time.Millisecond, alternate: true}
	BlueFlash    = CalendarState{Name: "Red-Blue Flash", primary: LightState{Blue: 255}, secondary: LightState{Red: 255}, primaryFlash: 500 * time.Millisecond, alternate: true}
	Blue         = CalendarState{Name: "Blue", primary: LightState{Blue: 255}, secondary: LightState{Blue: 255}}
	MagentaFlash = CalendarState{Name: "MagentaFlash", primary: LightState{Red: 255, Blue: 255}, secondary: offLightState, primaryFlash: 125 * time.Millisecond, alternate: true}
)

// Combines the two states into one state that shows both events.
func CombineStates(in1 CalendarState, in2 CalendarState) CalendarState {
	return CalendarState{
		Name:           in1.Name + "/" + in2.Name,
		primary:        in1.primary,
		secondary:      in2.primary,
		primaryFlash:   in1.primaryFlash,
		secondaryFlash: in2.primaryFlash,
		alternate:      false,
	}
}

// Swaps the sides for a state, for use in flashing.
func SwapState(in CalendarState) CalendarState {
	return CalendarState{
		Name:           in.Name + " swapped",
		primary:        in.secondary,
		secondary:      in.primary,
		primaryFlash:   in.secondaryFlash,
		secondaryFlash: in.primaryFlash,
		alternate:      false,
	}
}

// blinkerState encapsulates the current device state of the blink(1).
type BlinkerState struct {
	device      blinkDevice
	newState    chan CalendarState
	failures    int
	maxFailures int
	done        chan struct{}
	doneOnce    sync.Once
}

func NewBlinkerState(maxFailures int) *BlinkerState {
	blinker := &BlinkerState{
		newState:    make(chan CalendarState, 1),
		maxFailures: maxFailures,
		done:        make(chan struct{}),
	}
	if err := blinker.reinitialize(); err != nil {
		errorLog("Unable to initialize blink(1): %v\n", err)
	} else {
		blinker.playStartupSequence()
	}
	return blinker
}

func (blinker *BlinkerState) reinitialize() error {
	if blinker.device != nil {
		blinker.device.Close()
		blinker.device = nil
	}
	device, err := openBlinkDevice()
	if err != nil {
		blinker.failures++
		if blinker.maxFailures > 0 && blinker.failures == blinker.maxFailures {
			errorLog("Unable to initialize blink(1) after %d consecutive attempts: %v. Continuing to retry.\n", blinker.failures, err)
		}
		printDot("X")
		return err
	}
	if blinker.failures > 0 {
		log.Printf("Recovered blink(1) device after %d failed attempts\n", blinker.failures)
	}
	blinker.failures = 0
	blinker.device = device
	return nil
}

func (blinker *BlinkerState) turnOff() {
	if blinker.device != nil {
		_ = blinker.device.SetState(offLightState)
	}
}

func (blinker *BlinkerState) playStartupSequence() {
	steps := []LightState{
		{Red: 255, LED: LEDAll, FadeTime: 40 * time.Millisecond},
		{Green: 255, LED: LEDAll, FadeTime: 40 * time.Millisecond},
		{Blue: 255, LED: LEDAll, FadeTime: 40 * time.Millisecond},
		{LED: LEDAll, FadeTime: 40 * time.Millisecond},
	}

	for _, state := range steps {
		if err := blinker.setState(state); err != nil {
			debugLog("Startup blink sequence failed: %v\n", err)
			return
		}
		time.Sleep(startupSignalStep)
	}
}

func (blinker *BlinkerState) shutdown() {
	blinker.doneOnce.Do(func() {
		close(blinker.done)
		blinker.turnOff()
		if blinker.device != nil {
			blinker.device.Close()
			blinker.device = nil
		}
	})
}

func (blinker *BlinkerState) setState(state LightState) error {
	select {
	case <-blinker.done:
		return nil
	default:
	}

	if blinker.device == nil || blinker.failures > 0 {
		if err := blinker.reinitialize(); err != nil {
			errorLog("Reinitialize failed, error %v\n", err)
			return err
		}
	}
	if blinker.device == nil {
		return fmt.Errorf("blink(1) device unavailable")
	}

	err := blinker.device.SetState(state)
	if err != nil {
		errorLog("Re-initializing because of error %v\n", err)
		if err = blinker.reinitialize(); err != nil {
			errorLog("Reinitialize failed, error %v\n", err)
			return err
		}
		err = blinker.device.SetState(state)
		if err != nil {
			errorLog("Setting blinker state failed, error %v\n", err)
			return err
		}
	}
	blinker.failures = 0
	return nil
}

func (blinker *BlinkerState) patternRunner() {
	currentState := Black
	failing := false
	if err := blinker.setState(currentState.primary); err != nil {
		failing = true
	}

	var ticker <-chan time.Time
	stateFlip := false
	for {
		select {
		case <-blinker.done:
			return
		case newState := <-blinker.newState:
			if newState != currentState || failing {
				debugLog("Changing from state %v to %v\n", currentState, newState)
				currentState = newState
				if newState.primaryFlash > 0 || newState.secondaryFlash > 0 {
					ticker = time.After(time.Millisecond)
				} else {
					ticker = nil
					state1 := newState.primary
					state1.LED = LED1
					state2 := newState.secondary
					state2.LED = LED2
					err1 := blinker.setState(state1)
					err2 := blinker.setState(state2)
					failing = err1 != nil || err2 != nil
				}
			} else {
				debugLog("Retaining state %v unchanged\n", newState)
			}
		case <-ticker:
			verboseLog("Timer fired\n")
			state1 := currentState.primary
			state2 := currentState.secondary
			if stateFlip {
				if currentState.alternate {
					state1, state2 = state2, state1
				} else {
					if currentState.primaryFlash > 0 {
						state1 = offLightState
					}
					if currentState.secondaryFlash > 0 {
						state2 = offLightState
					}
				}
			}
			state1.Duration = currentState.primaryFlash
			state1.FadeTime = state1.Duration
			if currentState.alternate {
				state2.Duration, state2.FadeTime = state1.Duration, state1.FadeTime
			} else {
				state2.Duration = currentState.secondaryFlash
				state2.FadeTime = state2.Duration
			}
			state1.LED = LED1
			state2.LED = LED2
			verboseLog("Setting state (%v and %v)\n", state1, state2)
			err1 := blinker.setState(state1)
			err2 := blinker.setState(state2)
			failing = err1 != nil || err2 != nil
			stateFlip = !stateFlip
			nextTick := state1.Duration
			if nextTick == 0 {
				nextTick = state2.Duration
			}
			verboseLog("Next tick: %s\n", nextTick)
			ticker = time.After(nextTick)
		}
	}
}

// Signal handler - SIGINT and SIGTERM should turn off the blinker before exiting.
// SIGQUIT should turn on debug mode.
func signalHandler(blinker *BlinkerState, stop func()) {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)
	for {
		s := <-interrupt
		if s == syscall.SIGQUIT {
			fmt.Println("Turning on debug mode.")
			debug = debugOn
			continue
		}
		blinker.turnOff()
		log.Printf("Shutting down due to signal %v\n", s)
		stop()
		return
	}
}
