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

// This file manages network authentication and retrieval.

package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

const oauthTimeout = 5 * time.Minute

var oauthScopes = []string{
	calendar.CalendarEventsReadonlyScope,
	calendar.CalendarCalendarlistReadonlyScope,
}

type OAuthConfigSource string

const (
	OAuthSourceEnv          OAuthConfigSource = "environment"
	OAuthSourceConfig       OAuthConfigSource = "config"
	OAuthSourceOAuthJSON    OAuthConfigSource = "oauth-client.json"
	OAuthSourceClientSecret OAuthConfigSource = "client_secret.json"
)

type OAuthSettings struct {
	Config *oauth2.Config
	Source OAuthConfigSource
}

type authResponse struct {
	code  string
	err   error
	state string
}

type persistedTokenSource struct {
	ctx         context.Context
	config      *oauth2.Config
	tokenPath   string
	interactive bool
	current     *oauth2.Token
}

func Connect(paths AppPaths, userPrefs *UserPrefs, interactive bool) (*calendar.Service, error) {
	settings, err := loadOAuthSettings(paths, userPrefs)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	client, err := getClient(ctx, settings.Config, paths.TokenFile, interactive)
	if err != nil {
		return nil, err
	}

	srv, err := calendar.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve Calendar client: %w", err)
	}
	return srv, nil
}

func loadOAuthSettings(paths AppPaths, userPrefs *UserPrefs) (*OAuthSettings, error) {
	if visitedFlags["clientsecret"] {
		config, err := loadOAuthConfigFromFile(paths.ClientSecretFile, true)
		if err != nil {
			return nil, err
		}
		return &OAuthSettings{Config: config, Source: OAuthSourceClientSecret}, nil
	}

	if config, ok, err := loadOAuthConfigFromEnv(); err != nil {
		return nil, err
	} else if ok {
		return &OAuthSettings{Config: config, Source: OAuthSourceEnv}, nil
	}

	if config, ok, err := loadOAuthConfigFromPrefs(userPrefs); err != nil {
		return nil, err
	} else if ok {
		return &OAuthSettings{Config: config, Source: OAuthSourceConfig}, nil
	}

	if fileExists(paths.OAuthClientFile) {
		config, err := loadOAuthConfigFromFile(paths.OAuthClientFile, false)
		if err != nil {
			return nil, err
		}
		return &OAuthSettings{Config: config, Source: OAuthSourceOAuthJSON}, nil
	}

	if fileExists(paths.ClientSecretFile) {
		config, err := loadOAuthConfigFromFile(paths.ClientSecretFile, true)
		if err != nil {
			return nil, err
		}
		return &OAuthSettings{Config: config, Source: OAuthSourceClientSecret}, nil
	}

	return nil, fmt.Errorf("no Google OAuth client configuration found; set CALBLINK_GOOGLE_CLIENT_ID/CALBLINK_GOOGLE_CLIENT_SECRET, add googleClientID/googleClientSecret to config, or provide oauth-client.json/client_secret.json")
}

func loadOAuthConfigFromEnv() (*oauth2.Config, bool, error) {
	clientID := strings.TrimSpace(os.Getenv("CALBLINK_GOOGLE_CLIENT_ID"))
	clientSecret := strings.TrimSpace(os.Getenv("CALBLINK_GOOGLE_CLIENT_SECRET"))
	if clientID == "" && clientSecret == "" {
		return nil, false, nil
	}
	if clientID == "" || clientSecret == "" {
		return nil, false, fmt.Errorf("both CALBLINK_GOOGLE_CLIENT_ID and CALBLINK_GOOGLE_CLIENT_SECRET must be set")
	}
	return newInstalledAppOAuthConfig(clientID, clientSecret), true, nil
}

func loadOAuthConfigFromPrefs(userPrefs *UserPrefs) (*oauth2.Config, bool, error) {
	clientID := strings.TrimSpace(userPrefs.GoogleClientID)
	clientSecret := strings.TrimSpace(userPrefs.GoogleClientSecret)
	if clientID == "" && clientSecret == "" {
		return nil, false, nil
	}
	if clientID == "" || clientSecret == "" {
		return nil, false, fmt.Errorf("both googleClientID and googleClientSecret must be set in config")
	}
	return newInstalledAppOAuthConfig(clientID, clientSecret), true, nil
}

func newInstalledAppOAuthConfig(clientID, clientSecret string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint:     google.Endpoint,
		Scopes:       oauthScopes,
	}
}

func loadOAuthConfigFromFile(path string, requireSecurePermissions bool) (*oauth2.Config, error) {
	content, err := loadOAuthClientJSON(path, requireSecurePermissions)
	if err != nil {
		return nil, err
	}
	config, err := google.ConfigFromJSON(content, oauthScopes...)
	if err != nil {
		return nil, fmt.Errorf("unable to parse OAuth client config %s: %w", path, err)
	}
	return config, nil
}

func loadOAuthClientJSON(path string, requireSecurePermissions bool) ([]byte, error) {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("oauth client file not found: %s", path)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to stat oauth client file: %w", err)
	}
	if requireSecurePermissions && info.Mode().Perm()&077 != 0 {
		return nil, fmt.Errorf("insecure permissions for oauth client file: %s", path)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read oauth client file: %w", err)
	}
	return content, nil
}

func getClient(ctx context.Context, config *oauth2.Config, tokenPath string, interactive bool) (*http.Client, error) {
	source, err := newPersistedTokenSource(ctx, config, tokenPath, interactive)
	if err != nil {
		return nil, err
	}
	return oauth2.NewClient(ctx, source), nil
}

func newPersistedTokenSource(ctx context.Context, config *oauth2.Config, tokenPath string, interactive bool) (oauth2.TokenSource, error) {
	source := &persistedTokenSource{
		ctx:         ctx,
		config:      config,
		tokenPath:   tokenPath,
		interactive: interactive,
	}
	if tok, err := tokenFromFile(tokenPath); err == nil {
		source.current = tok
	}
	if _, err := source.Token(); err != nil {
		return nil, err
	}
	return source, nil
}

func (s *persistedTokenSource) Token() (*oauth2.Token, error) {
	if s.current != nil && s.current.Valid() {
		return s.current, nil
	}

	if s.current != nil && s.current.RefreshToken != "" {
		refreshed, err := s.config.TokenSource(s.ctx, s.current).Token()
		if err == nil {
			if refreshed.RefreshToken == "" {
				refreshed.RefreshToken = s.current.RefreshToken
			}
			if err := saveToken(s.tokenPath, refreshed); err != nil {
				return nil, err
			}
			s.current = refreshed
			return refreshed, nil
		}
		if !shouldTriggerReauth(err) {
			return nil, err
		}
		debugLog("OAuth refresh failed, falling back to browser auth: %v\n", err)
	}

	if !s.interactive {
		if s.current == nil {
			return nil, fmt.Errorf("no cached OAuth token available and interactive authentication is disabled")
		}
		return nil, fmt.Errorf("cached OAuth token could not be refreshed and interactive authentication is disabled")
	}

	token, err := getTokenFromWeb(s.ctx, s.config)
	if err != nil {
		return nil, err
	}
	if err := saveToken(s.tokenPath, token); err != nil {
		return nil, err
	}
	s.current = token
	return token, nil
}

func shouldTriggerReauth(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "invalid_grant") ||
		strings.Contains(message, "expired or revoked") ||
		strings.Contains(message, "token has been expired or revoked")
}

func getTokenFromWeb(ctx context.Context, config *oauth2.Config) (*oauth2.Token, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("unable to start local callback listener: %w", err)
	}
	defer func() {
		_ = listener.Close()
	}()

	state, err := randomState()
	if err != nil {
		return nil, err
	}
	config.RedirectURL = fmt.Sprintf("http://%s/callback", listener.Addr().String())
	authURL := config.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce)

	responseCh := make(chan authResponse, 1)
	server := &http.Server{
		Handler: newAuthHandler(responseCh),
	}

	go func() {
		if serveErr := server.Serve(listener); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			responseCh <- authResponse{err: serveErr}
		}
	}()

	fmt.Printf("Opening a browser to authorize calblink.\nIf it does not open automatically, visit:\n%v\n", authURL)
	openBrowser(authURL)

	timeoutCtx, cancel := context.WithTimeout(ctx, oauthTimeout)
	defer cancel()

	var response authResponse
	select {
	case response = <-responseCh:
	case <-timeoutCtx.Done():
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		_ = server.Shutdown(shutdownCtx)
		return nil, fmt.Errorf("timed out waiting for OAuth response")
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	_ = server.Shutdown(shutdownCtx)

	if response.err != nil {
		return nil, fmt.Errorf("unable to retrieve token from web: %w", response.err)
	}
	if response.state != state {
		return nil, fmt.Errorf("oauth state mismatch")
	}
	tok, err := config.Exchange(ctx, response.code)
	if err != nil {
		return nil, fmt.Errorf("unable to exchange token: %w", err)
	}
	return tok, nil
}

func newAuthHandler(responseCh chan<- authResponse) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/callback" {
			http.NotFound(w, req)
			return
		}

		query := req.URL.Query()
		if errValue := query.Get("error"); errValue != "" {
			http.Error(w, "Authorization failed. You can close this window.", http.StatusBadRequest)
			responseCh <- authResponse{err: fmt.Errorf("authorization failed: %s", errValue)}
			return
		}

		code := query.Get("code")
		if code == "" {
			http.Error(w, "Missing authorization code. You can close this window.", http.StatusBadRequest)
			responseCh <- authResponse{err: fmt.Errorf("missing authorization code")}
			return
		}

		_, _ = fmt.Fprint(w, "Token received. You can close this window.")
		responseCh <- authResponse{
			code:  code,
			state: query.Get("state"),
		}
	})
}

func randomState() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("unable to generate oauth state: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func openBrowser(target string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", target)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", target)
	default:
		cmd = exec.Command("xdg-open", target)
	}
	if err := cmd.Start(); err != nil {
		debugLog("Unable to open browser automatically: %v\n", err)
	}
}

func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = f.Close()
	}()
	t := &oauth2.Token{}
	if err := json.NewDecoder(f).Decode(t); err != nil {
		return nil, err
	}
	return t, nil
}

func saveToken(file string, token *oauth2.Token) error {
	if err := os.MkdirAll(filepath.Dir(file), 0700); err != nil {
		return fmt.Errorf("unable to create token cache directory: %w", err)
	}
	f, err := os.OpenFile(file, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("unable to cache oauth token: %w", err)
	}
	defer func() {
		_ = f.Close()
	}()
	if err := json.NewEncoder(f).Encode(token); err != nil {
		return fmt.Errorf("unable to encode oauth token: %w", err)
	}
	return nil
}

func removeToken(file string) error {
	if err := os.Remove(file); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func printAuthStatus(paths AppPaths) error {
	token, err := tokenFromFile(paths.TokenFile)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No cached Google OAuth token found.")
			return nil
		}
		return err
	}

	fmt.Printf("OAuth token file: %s\n", paths.TokenFile)
	if token.Expiry.IsZero() {
		fmt.Println("Access token expiry: unknown")
	} else {
		fmt.Printf("Access token expiry: %s\n", token.Expiry.Format(time.RFC3339))
	}
	if token.RefreshToken != "" {
		fmt.Println("Refresh token: available")
	} else {
		fmt.Println("Refresh token: missing")
	}
	return nil
}

func runAuthAction(action string, paths AppPaths, userPrefs *UserPrefs) error {
	switch action {
	case "login":
		settings, err := loadOAuthSettings(paths, userPrefs)
		if err != nil {
			return err
		}
		_, err = newPersistedTokenSource(context.Background(), settings.Config, paths.TokenFile, true)
		if err != nil {
			return err
		}
		fmt.Printf("Google Calendar auth is ready using %s credentials.\n", settings.Source)
		return nil
	case "logout":
		if err := removeToken(paths.TokenFile); err != nil {
			return err
		}
		fmt.Println("Removed cached Google OAuth token.")
		return nil
	case "status":
		return printAuthStatus(paths)
	default:
		return fmt.Errorf("unknown auth action %q; expected login, logout, or status", action)
	}
}
