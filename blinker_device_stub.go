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

//go:build !blink1

package main

import (
	"errors"
	"fmt"
)

var errBlinkDeviceUnavailable = errors.New("blink(1) support is not compiled in; rebuild with -tags blink1")

func openBlinkDevice() (blinkDevice, error) {
	return nil, errBlinkDeviceUnavailable
}

func printBlinkDeviceDebugInfo() error {
	return fmt.Errorf("%w; rerun with -tags blink1", errBlinkDeviceUnavailable)
}
