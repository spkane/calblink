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

func TestResolveAppPathsFindsLegacyOAuthClient(t *testing.T) {
	homeDir := t.TempDir()
	workDir := t.TempDir()
	xdgConfigDir := filepath.Join(homeDir, ".config")
	legacyDir := filepath.Join(homeDir, ".calblink")
	legacyOAuthClient := filepath.Join(legacyDir, "oauth-client.json")

	t.Setenv("HOME", homeDir)
	t.Setenv("XDG_CONFIG_HOME", xdgConfigDir)
	t.Chdir(workDir)
	if err := os.MkdirAll(legacyDir, 0700); err != nil {
		t.Fatalf("failed to create legacy config dir: %v", err)
	}
	if err := os.WriteFile(legacyOAuthClient, []byte("{}"), 0600); err != nil {
		t.Fatalf("failed to write legacy OAuth client file: %v", err)
	}

	visitedFlags = map[string]bool{}
	paths := resolveAppPaths()

	if paths.OAuthClientFile != legacyOAuthClient {
		t.Fatalf("OAuthClientFile = %q, want %q", paths.OAuthClientFile, legacyOAuthClient)
	}
}

func TestResolveAppPathsFindsLegacyConfig(t *testing.T) {
	homeDir := t.TempDir()
	workDir := t.TempDir()
	xdgConfigDir := filepath.Join(homeDir, ".config")
	legacyDir := filepath.Join(homeDir, ".calblink")
	legacyConfig := filepath.Join(legacyDir, "config.toml")

	t.Setenv("HOME", homeDir)
	t.Setenv("XDG_CONFIG_HOME", xdgConfigDir)
	t.Chdir(workDir)
	if err := os.MkdirAll(legacyDir, 0700); err != nil {
		t.Fatalf("failed to create legacy config dir: %v", err)
	}
	if err := os.WriteFile(legacyConfig, []byte("pollInterval = 60\n"), 0600); err != nil {
		t.Fatalf("failed to write legacy config file: %v", err)
	}

	visitedFlags = map[string]bool{}
	paths := resolveAppPaths()

	if paths.ConfigFile != legacyConfig {
		t.Fatalf("ConfigFile = %q, want %q", paths.ConfigFile, legacyConfig)
	}
}

func TestResolveAppPathsFindsLegacyToken(t *testing.T) {
	homeDir := t.TempDir()
	workDir := t.TempDir()
	xdgConfigDir := filepath.Join(homeDir, ".config")
	legacyDir := filepath.Join(homeDir, ".calblink")
	legacyToken := filepath.Join(legacyDir, "token.json")

	t.Setenv("HOME", homeDir)
	t.Setenv("XDG_CONFIG_HOME", xdgConfigDir)
	t.Chdir(workDir)
	if err := os.MkdirAll(legacyDir, 0700); err != nil {
		t.Fatalf("failed to create legacy config dir: %v", err)
	}
	if err := os.WriteFile(legacyToken, []byte("{}"), 0600); err != nil {
		t.Fatalf("failed to write legacy token file: %v", err)
	}

	visitedFlags = map[string]bool{}
	paths := resolveAppPaths()

	if paths.TokenFile != legacyToken {
		t.Fatalf("TokenFile = %q, want %q", paths.TokenFile, legacyToken)
	}
}
