// Copyright 2026 calblink contributors.
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

//go:build blink1

package main

import (
	"errors"
	"fmt"
	"sync"
	"time"

	hid "github.com/sstallion/go-hid"
)

const (
	blink1VendorID        = 0x27b8
	blink1ProductID       = 0x01ed
	blink1FeatureReportID = 1
)

var (
	hidInitOnce sync.Once
	hidInitErr  error
)

type realBlinkDevice struct {
	device *hid.Device
}

func ensureHIDInitialized() error {
	hidInitOnce.Do(func() {
		hidInitErr = hid.Init()
	})
	return hidInitErr
}

func openBlinkDevice() (blinkDevice, error) {
	infos, err := enumerateBlinkDevices()
	if err != nil {
		return nil, err
	}
	if len(infos) == 0 {
		return nil, errors.New("no Blink(1) device found")
	}

	var lastErr error
	for _, info := range infos {
		device, err := openEnumeratedBlinkDevice(info)
		if err != nil {
			lastErr = fmt.Errorf("open blink(1) device %s: %w", blinkDeviceSummary(info), err)
			continue
		}
		debugLog("Opened blink(1) device via HID: %s\n", blinkDeviceSummary(info))
		return &realBlinkDevice{device: device}, nil
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, errors.New("no Blink(1) device found")
}

func enumerateBlinkDevices() ([]hid.DeviceInfo, error) {
	if err := ensureHIDInitialized(); err != nil {
		return nil, fmt.Errorf("initialize hidapi: %w", err)
	}

	var infos []hid.DeviceInfo
	if err := hid.Enumerate(blink1VendorID, blink1ProductID, func(info *hid.DeviceInfo) error {
		infos = append(infos, *info)
		return nil
	}); err != nil {
		return nil, fmt.Errorf("enumerate blink(1) HID devices: %w", err)
	}
	return infos, nil
}

func openEnumeratedBlinkDevice(info hid.DeviceInfo) (*hid.Device, error) {
	if err := ensureHIDInitialized(); err != nil {
		return nil, fmt.Errorf("initialize hidapi: %w", err)
	}
	if info.SerialNbr != "" {
		return hid.Open(info.VendorID, info.ProductID, info.SerialNbr)
	}
	return hid.OpenFirst(info.VendorID, info.ProductID)
}

func printBlinkDeviceDebugInfo() error {
	infos, err := enumerateBlinkDevices()
	if err != nil {
		return err
	}
	if len(infos) == 0 {
		fmt.Println("No blink(1) HID devices found.")
		return nil
	}

	fmt.Printf("Found %d blink(1) HID device(s):\n", len(infos))
	for i, info := range infos {
		fmt.Printf("[%d] %s\n", i+1, blinkDeviceSummary(info))
		device, err := openEnumeratedBlinkDevice(info)
		if err != nil {
			fmt.Printf("    open: FAILED: %v\n", err)
			continue
		}
		fmt.Printf("    open: ok\n")
		if liveInfo, err := device.GetDeviceInfo(); err == nil && liveInfo != nil {
			fmt.Printf("    opened-as: %s\n", blinkDeviceSummary(*liveInfo))
		}
		_ = device.Close()
	}
	return nil
}

func blinkDeviceSummary(info hid.DeviceInfo) string {
	return fmt.Sprintf(
		"path=%q serial=%q manufacturer=%q product=%q release=0x%04x usagePage=0x%04x usage=0x%04x interface=%d bus=%v",
		info.Path,
		info.SerialNbr,
		info.MfrStr,
		info.ProductStr,
		info.ReleaseNbr,
		info.UsagePage,
		info.Usage,
		info.InterfaceNbr,
		info.BusType,
	)
}

func (d *realBlinkDevice) Close() {
	if d.device != nil {
		_ = d.device.Close()
		d.device = nil
	}
}

func (d *realBlinkDevice) SetState(state LightState) error {
	if d.device == nil {
		return errors.New("blink(1) device is not open")
	}

	data := blink1FadeToRGBReport(state)
	n, err := d.device.SendFeatureReport(data[:])
	if err != nil {
		return fmt.Errorf("write blink(1) state: %w", err)
	}
	if n != len(data) {
		return fmt.Errorf("write blink(1) state: short write (%d/%d)", n, len(data))
	}

	return nil
}

func blink1FadeToRGBReport(state LightState) [8]byte {
	dms := int(state.FadeTime / (10 * time.Millisecond))
	return [8]byte{
		blink1FeatureReportID,
		'c',
		state.Red,
		state.Green,
		state.Blue,
		byte(dms >> 8),
		byte(dms & 0xff),
		state.LED,
	}
}
