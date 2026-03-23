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

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadTomlPrefsKeepsDefaultShowDotsWhenOmitted(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(configPath, []byte("pollInterval = 60\n"), 0600); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	prefs := readTomlPrefs(configPath)
	if !prefs.ShowDots {
		t.Fatal("expected showDots default to remain true when omitted from TOML")
	}
	if prefs.PollInterval != 60 {
		t.Fatalf("expected pollInterval override to apply, got %d", prefs.PollInterval)
	}
}

func TestParseBoolPrefSupportsLegacyStringAndBoolValues(t *testing.T) {
	cases := []struct {
		name  string
		input any
		value bool
		ok    bool
	}{
		{name: "bool true", input: true, value: true, ok: true},
		{name: "string false", input: "false", value: false, ok: true},
		{name: "missing", input: nil, value: false, ok: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			value, ok := parseBoolPref(tc.input)
			if value != tc.value || ok != tc.ok {
				t.Fatalf("parseBoolPref(%v) = (%v, %v), want (%v, %v)", tc.input, value, ok, tc.value, tc.ok)
			}
		})
	}
}
