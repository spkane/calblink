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
)

type AppPaths struct {
	ConfigDir        string
	ConfigFile       string
	LegacyConfigFile string
	OAuthClientFile  string
	ClientSecretFile string
	TokenFile        string
}

var (
	appPaths     AppPaths
	visitedFlags map[string]bool
)

func resolveAppPaths() AppPaths {
	workingDir, err := os.Getwd()
	if err != nil {
		workingDir = "."
	}

	configDir := defaultConfigDir(workingDir)
	legacyConfigDir := defaultLegacyConfigDir(workingDir)
	localConfig := filepath.Join(workingDir, "conf.toml")
	localLegacyConfig := filepath.Join(workingDir, "conf.json")
	defaultConfig := filepath.Join(configDir, "config.toml")
	defaultLegacyConfig := filepath.Join(configDir, "conf.json")
	legacyConfig := filepath.Join(legacyConfigDir, "config.toml")
	legacyLegacyConfig := filepath.Join(legacyConfigDir, "conf.json")
	defaultOAuthClient := filepath.Join(configDir, "oauth-client.json")
	legacyOAuthClient := filepath.Join(legacyConfigDir, "oauth-client.json")
	defaultClientSecret := filepath.Join(configDir, "client_secret.json")
	legacyClientSecret := filepath.Join(legacyConfigDir, "client_secret.json")
	localClientSecret := filepath.Join(workingDir, "client_secret.json")
	localOAuthClient := filepath.Join(workingDir, "oauth-client.json")
	defaultToken := filepath.Join(configDir, "token.json")
	legacyToken := filepath.Join(legacyConfigDir, "token.json")

	configPath := defaultConfig
	switch {
	case visitedFlags["config"]:
		configPath = cleanPath(*configFileFlag)
	case fileExists(localConfig):
		configPath = localConfig
	case fileExists(defaultConfig):
		configPath = defaultConfig
	case fileExists(legacyConfig):
		configPath = legacyConfig
	case fileExists(localLegacyConfig):
		configPath = localLegacyConfig
	case fileExists(defaultLegacyConfig):
		configPath = defaultLegacyConfig
	case fileExists(legacyLegacyConfig):
		configPath = legacyLegacyConfig
	}

	legacyConfigPath := defaultLegacyConfig
	if visitedFlags["backup_config"] {
		legacyConfigPath = cleanPath(*backupConfigFileFlag)
	} else if fileExists(localLegacyConfig) {
		legacyConfigPath = localLegacyConfig
	} else if fileExists(defaultLegacyConfig) {
		legacyConfigPath = defaultLegacyConfig
	} else if fileExists(legacyLegacyConfig) {
		legacyConfigPath = legacyLegacyConfig
	}

	clientSecretPath := defaultClientSecret
	switch {
	case visitedFlags["clientsecret"]:
		clientSecretPath = cleanPath(*clientSecretFlag)
	case fileExists(localClientSecret):
		clientSecretPath = localClientSecret
	case fileExists(defaultClientSecret):
		clientSecretPath = defaultClientSecret
	case fileExists(legacyClientSecret):
		clientSecretPath = legacyClientSecret
	}

	return AppPaths{
		ConfigDir:        configDir,
		ConfigFile:       configPath,
		LegacyConfigFile: legacyConfigPath,
		OAuthClientFile:  firstExistingPath(localOAuthClient, defaultOAuthClient, legacyOAuthClient),
		ClientSecretFile: clientSecretPath,
		TokenFile:        firstExistingPath(defaultToken, legacyToken),
	}
}

func defaultConfigDir(fallback string) string {
	baseDir, err := os.UserConfigDir()
	if err != nil || baseDir == "" {
		return defaultLegacyConfigDir(fallback)
	}
	return filepath.Join(baseDir, "calblink")
}

func defaultLegacyConfigDir(fallback string) string {
	homeDir, err := os.UserHomeDir()
	if err != nil || homeDir == "" {
		return filepath.Join(fallback, ".calblink")
	}
	return filepath.Join(homeDir, ".calblink")
}

func cleanPath(path string) string {
	if path == "" {
		return path
	}
	return filepath.Clean(path)
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func firstExistingPath(paths ...string) string {
	for _, path := range paths {
		if fileExists(path) {
			return path
		}
	}
	if len(paths) == 0 {
		return ""
	}
	return paths[0]
}
