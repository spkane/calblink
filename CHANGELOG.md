# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [v2.0.0] - 2026-03-23

### Added

- Added a modern browser-based OAuth flow with local callback handling instead of the older one-shot credential flow.
- Added `-auth login`, `-auth logout`, and `-auth status` commands for managing cached Google Calendar authentication.
- Added `-list_calendars` to print the calendars visible to the authenticated account.
- Added `-list_blink1_devices` to print blink(1) HID enumeration details and verify whether each device can be opened.
- Added support for loading OAuth client credentials from environment variables, config values, `oauth-client.json`, or legacy `client_secret.json`.
- Added a platform-aware per-user config path resolver and support for `config.toml` in the user config directory.
- Added a dedicated HID-based blink(1) backend with build-tag support and a stub backend when blink(1) support is not compiled in.
- Added a one-time startup RGB sequence so the blink(1) visibly confirms successful startup.
- Added release automation for tagged builds, including multi-platform archive generation and checksum publishing.
- Added Go tests for calendar resolution, config handling, and OAuth/network behavior.
- Added repository tooling for `pre-commit`, `golangci-lint`, markdown linting, and release hygiene.

### Changed

- Reworked authentication to use refreshable cached OAuth tokens and automatically re-open the browser only when re-consent is actually needed.
- Changed calendar configuration so calendar entries can be resolved by ID or by display name, with clearer errors for missing or ambiguous matches.
- Improved multi-calendar support by resolving configured references against the authenticated account's visible calendar list before monitoring begins.
- Updated blink(1) device access to use HID APIs across platforms rather than direct USB interface claiming.
- Updated service handling to pass through resolved config and credential paths more cleanly and to shut down more gracefully.
- Updated README and service documentation to cover the new OAuth setup, modern config locations, multi-account calendar sharing, blink(1) diagnostics, and current build instructions.
- Updated development tooling and hook configuration to current stable pre-commit hook releases.

### Fixed

- Fixed macOS blink(1) access failures caused by the older USB/libusb-based implementation.
- Fixed startup and shutdown handling for service-managed runs by using the shared exit path instead of closing the exit channel directly.
- Fixed stale config-path assumptions by moving config, OAuth client, and token discovery into a consistent path-resolution layer.
- Fixed markdown and repo hygiene issues so `pre-commit run --all-files` now passes cleanly.
- Fixed service-mode documentation formatting and examples.
- Fixed repeated crash-loop behavior caused by older launch/service assumptions around missing legacy config paths and older blink(1) access methods.
