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
	"errors"
	"os"
	"testing"
)

func TestLoadOAuthConfigFromPrefs(t *testing.T) {
	prefs := getDefaultPrefs()
	prefs.GoogleClientID = "client-id"
	prefs.GoogleClientSecret = "client-secret"

	config, ok, err := loadOAuthConfigFromPrefs(prefs)
	if err != nil {
		t.Fatalf("loadOAuthConfigFromPrefs returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected config to be loaded from prefs")
	}
	if config.ClientID != "client-id" || config.ClientSecret != "client-secret" {
		t.Fatalf("unexpected config values: %#v", config)
	}
}

func TestLoadOAuthConfigFromEnvRequiresBothValues(t *testing.T) {
	t.Setenv("CALBLINK_GOOGLE_CLIENT_ID", "client-id")
	t.Setenv("CALBLINK_GOOGLE_CLIENT_SECRET", "")

	if _, ok, err := loadOAuthConfigFromEnv(); err == nil || ok {
		t.Fatal("expected partial env configuration to fail")
	}
}

func TestLoadOAuthSettingsPrefersEnvOverPrefs(t *testing.T) {
	t.Setenv("CALBLINK_GOOGLE_CLIENT_ID", "env-client-id")
	t.Setenv("CALBLINK_GOOGLE_CLIENT_SECRET", "env-client-secret")

	prefs := getDefaultPrefs()
	prefs.GoogleClientID = "config-client-id"
	prefs.GoogleClientSecret = "config-client-secret"

	paths := AppPaths{
		ConfigDir:        t.TempDir(),
		ConfigFile:       "config.toml",
		LegacyConfigFile: "conf.json",
		OAuthClientFile:  "oauth-client.json",
		ClientSecretFile: "client_secret.json",
		TokenFile:        "token.json",
	}

	visitedFlags = map[string]bool{}
	settings, err := loadOAuthSettings(paths, prefs)
	if err != nil {
		t.Fatalf("loadOAuthSettings returned error: %v", err)
	}
	if settings.Source != OAuthSourceEnv {
		t.Fatalf("expected environment credentials to win, got %s", settings.Source)
	}
}

func TestRemoveTokenIgnoresMissingFile(t *testing.T) {
	if err := removeToken(os.TempDir() + "/definitely-not-a-real-calblink-token.json"); err != nil {
		t.Fatalf("removeToken should ignore missing files, got %v", err)
	}
}

func TestShouldTriggerReauth(t *testing.T) {
	if !shouldTriggerReauth(errors.New("oauth2: invalid_grant")) {
		t.Fatal("expected invalid_grant to trigger reauth")
	}
	if shouldTriggerReauth(errors.New("dial tcp: i/o timeout")) {
		t.Fatal("expected transient network errors not to trigger reauth")
	}
}
