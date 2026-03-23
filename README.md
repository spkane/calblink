# Blink(1) for Google Calendar (calblink)

## What is this?

Calblink is a small program to watch your Google Calendar and set a blink(1) USB
LED to change colors based on your next meeting. The colors it will use are:

- Off: nothing on your calendar for the next 60 minutes
- Green: 30 to 60 minutes
- Yellow: 10 to 30 minutes
- Red: 5 to 10 minutes
- Flashing red: 0 to 5 minutes, flashing faster for the last 2 minutes
- Flashing blue and red: First minute of the meeting
- Blue: In meeting
- Flashing magenta: Unable to connect to Calendar server.  This is to prevent
    the case where calblink silently fails and leaves you unaware that it has
    failed.

## What do I need use it?

To use calblink, you need the following:

1. A blink(1) from [ThingM](http://blink1.thingm.com/) - calblink supports
    mk1, mk2, and mk3 blink(1).
1. A place to connect the blink(1) where you can see it.
1. The latest stable version of [Go](https://go.dev/) supported by this repo.
    The project is currently pinned to Go 1.26.1 in CI and release automation.
1. The calblink code, found in this directory.
1. A working CGO toolchain for the blink(1) HID backend. On macOS this usually
    means the Xcode Command Line Tools.
1. A directory to run this in.
1. A few go packages, which we'll install later in the Setup section.
1. A Google Calendar account.
1. A Google Calendar desktop OAuth client. (We'll discuss the easiest ways to
    provide that in the Setup section.)

## How do I set this up?

1. Install Go, and plug your blink(1) in somewhere that you can see it.
2. Bring up a command-line window, and create the directory you want to run
    this in.
3. Put all .go files in this repo into the directory you just created.
4. Install native build tooling for CGO, if needed.
    - macOS: `xcode-select --install` is usually enough.
    - Linux: install your normal C toolchain package set for Go CGO builds.

5. Install dependencies:

    ```bash
    go mod tidy
    ```

6. If you already have a `go.mod` and `go.sum`, you may need to update them
    before compiling.

    ```bash
    go get -u
    go mod tidy
    ```

7. Create a desktop OAuth client as described in the [Google Calendar
    Quickstart](https://developers.google.com/workspace/calendar/api/quickstart/go).
    - If you are using a Workspace account and your org allows it, the simplest
        path is usually an Internal app.
    - If you are using a consumer Google account such as `@gmail.com`, use this
        current Google Auth Platform flow instead:
        1. In Google Cloud Console, open `Google Auth Platform`. If Google says
            the platform is not configured yet, click `Get Started`.
        2. On `Branding`, enter an app name and support email, then save. The
            branding and verification warnings are normal for a personal app.
        3. On `Audience`, choose `External`.
        4. If the app is in `Testing`, add the Google account(s) that will use
            calblink as `Test users`. This is where the old `+ Add Users` step
            lives now. If you do not see a test-user list, you are probably
            already in `In production` mode.
        5. On `Data Access`, add the Google Calendar read-only scopes used by
            calblink.
        6. Create the OAuth client under `Clients` as a `Desktop app`, then
            download the JSON file.
        7. Important: if you leave the app in `Testing`, Google expires test-user
            authorizations and refresh tokens after 7 days. For the "log in once
            and let calblink keep refreshing" workflow, switch the app to `In
            production` when you are ready.
    - For a personal-use app with fewer than 100 users, Google says OAuth
        verification is not mandatory, but users will see the unverified-app
        warning during sign-in. That is expected for a personal calblink setup.

8. Choose one of the following auth configuration options:
    - Recommended: put the downloaded desktop OAuth client JSON at
        `oauth-client.json` in the directory where you run calblink, or in your
        per-user calblink config directory.
    - Per-user config directory locations:
      - macOS: `~/Library/Application Support/calblink/oauth-client.json`
      - Linux: `$XDG_CONFIG_HOME/calblink/oauth-client.json` or
        `~/.config/calblink/oauth-client.json`
      - Windows: `%AppData%\calblink\oauth-client.json`
      - Fallback (only if the OS config directory is unavailable):
        `~/.calblink/oauth-client.json`
    - During Google sign-in you will likely see a "Google hasn't verified this
        app" warning because a personal calblink OAuth app is typically
        unverified. That is expected. Use the advanced/continue flow in Google
        to allow access anyway.
    - Config file: set `googleClientID` and `googleClientSecret` in your
        TOML config file.
    - Environment variables: set `CALBLINK_GOOGLE_CLIENT_ID` and
        `CALBLINK_GOOGLE_CLIENT_SECRET`.
    - Advanced/legacy: keep using `client_secret.json` and/or `--clientsecret`.

9. If you use the legacy `client_secret.json` path, make sure it is secure by
    changing its permissions to only allow the user to read it:

    ```bash
    chmod 600 client_secret.json
    ```

10. Build the calblink program:

    ```bash
    go build -tags blink1
    ```

    The `blink1` build tag enables the HID backend for the blink(1) device on
    macOS, Linux, and Windows. If CGO is not configured on your machine, install
    the native build tools for your platform first.

11. Authorize calblink once:

    ```bash
    ./calblink -auth login
    ```

    This opens a browser, asks you to sign in, and stores a refresh token in
    your per-user calblink config directory.

12. Run the calblink program:

    ```bash
    ./calblink
    ```

13. That's it! It should just run now, and set your blink(1) to change color
    appropriately. To quit out of it, hit Ctrl-C in the window you ran it in.
    (It will turn the blink(1) off automatically.) It will output a . into the
    terminal window every time it checks the server and sets the LED.

14. Calblink will automatically refresh access tokens in the background. If the
    refresh token is revoked or Google needs you to re-consent, calblink will
    reopen the browser only when necessary.

15. If you want to monitor calendars from multiple Google accounts, use one
    Google account as the calblink auth account and share the other calendars
    to it.

    - Sign in to Google Calendar on the web for each secondary account and
      share the calendar with the account that calblink will use.
    - Grant at least "See all event details". "See only free/busy" is not
      enough for calblink because it needs actual event timing and metadata.
    - In the calblink auth account, accept or subscribe to those shared
      calendars so they appear in Google Calendar.
    - Run `./calblink -auth login` and make sure you sign in as that one
      central account.
    - Run `./calblink -list_calendars` to see the names and IDs now visible to
      that account.
    - Add those calendar names or IDs to `calendars = [...]` in config.

    Calblink currently uses one cached OAuth token per config directory, so one
    process authenticates as one Google account. Multi-account monitoring works
    by giving that account access to the other calendars.

16. Optionally, set up a config file, as below.

17. Once everything is working, you can consider enabling [service mode](SERVICE.md) to
    have it run automatically in the background.

Prebuilt release archives are intended to be published for:

- Windows amd64
- Linux amd64
- Linux arm64
- macOS amd64
- macOS arm64

The blink(1) support path now uses HID APIs across platforms. Prebuilt release
archives should include the OS-appropriate HID implementation automatically.

## What are the configuration options?

First off, run it with the --help option to see what the command-line options
are. Useful, perhaps, but maybe not what you want to use every time you run it.

calblink will first look for `conf.toml` in the current directory. If it doesn't
find that, it will look in your per-user calblink config directory for
`config.toml`, then fall back to the legacy JSON config locations. The configuration
file includes several useful options you can set:

- excludes - a list of event titles which it will ignore. If you like blocking
    out time with "Make Time" or similar, you can add these names to the
    'excludes' array.
- excludePrefixes - a list of event title prefixes which it will ignore.  This is useful
    for blocks that start consistently but may not end consistently, such as "On call,
    secondary is PERSON".
- startTime - an HH:MM time (24-hour clock) which calblink won't turn on
    before. Because you might not want it turning on at 4am.
- endTime - an HH:MM time (24-hour clock) which it won't turn on after.
- skipDays - a list of days of the week that it should skip. A blink(1) in
    the offices doesn't need to run on Saturday/Sunday, after all, and if you
    WFH every Friday, why distract your coworkers?
- pollInterval - how often (in seconds) it should check with Calendar for an
    update. Default is 30 seconds. Don't push this too frequent or you'll run
    out of API quota.
- calendar - which calendar to watch (defaults to primary). This can be the
    calendar's ID/email address, or its display name as shown in Google Calendar.
    "primary" is a magic string that means "the main calendar of the account whose
    auth token I'm using". If you are combining calendars from multiple Google
    accounts, this still refers only to the primary calendar of the one account
    calblink authenticated as.
- calendars - array of calendars to watch.  This will override calendar if it is set.
    All calendars listed will be watched for events. Calendar entries may be IDs or
    display names. Note that the signed-in account must have access to all calendars,
    and that if you query too many calendars you may run into issues with the free
    query quota for Google Calendar, especially if you are using your oauth key in
    multiple locations. For calendars owned by other Google accounts, share or
    subscribe to them in the authenticated account first, then list them here.
- responseState - which response states are marked as being valid for a
    meeting. Can be set to "all", in which case any item on your calendar will
    light up; "accepted", in which case only items marked as 'accepted' on
    calendar will light up; or "notRejected", in which case items that you have
    rejected will not light up. Default is "notRejected".
- deviceFailureRetries - how many consecutive blink(1) initialization failures to
    tolerate before calblink starts logging the device as unavailable. Default is 10.
    Calblink will continue retrying so it can recover automatically if the device is
    reconnected.
- showDots - whether to show a dot (or similar mark) after every poll interval
    to show that the program is running. Default is true. Symbols have the
    following meanings:
  - . - working normally
  - , - unable to talk to the calendar server. After 3 consecutive failures,
         the blink(1) will be set to flashing magenta to indicate that it is no
         longer current.
  - < - sleeping because we've reached endTime for today.
  - \> - sleeping because we haven't reached startTime yet today.
  - ~ - sleeping because it's a skip day
  - X - device failure.
- multiEvent - if true, calblink will check the next two events, and if they are
    both in the time frame to show, it will show both.
- priorityFlashSide - if 0 (the default), which side of the blink(1) is flashing
    will not be adjusted.  If set to 1, then flashing will be prioritized on LED 1;
 if 2, flashing will be prioritized on LED2.  Any other values are undefined.
- workingLocations - a list of working locations to filter results by.  If all
    calendars with working locations set have locations that are not in the list of
    locations, no events will be shown.  Handling of multiple calendars with working
    locations set may be suboptimal - if one calendar is set to homeOffice and another
    is set to an office location, both will be valid for all events on either calendar.
    Values should be in the following formats:
  - 'home' to indicate WFH
  - 'office' to match any office working location
  - 'office:NAME' to match an office location called NAME.
  - 'custom' to match any custom working location
  - 'custom:NAME' to match a custom location called NAME.
- googleClientID - optional desktop OAuth client ID. This is the easiest config-file
    based alternative to `oauth-client.json`.
- googleClientSecret - optional desktop OAuth client secret to pair with
    `googleClientID`.

An example TOML file:

```toml
        excludes = ["Commute"]
        skipDays = ["Saturday", "Sunday"]
        startTime = "08:45"
        endTime = "18:00"
        pollInterval = 60
        calendars = ["primary", "username@example.com"]
        responseState = "accepted"
        multiEvent = true
        priorityFlashSide = 1
        workingLocations = ["home"]
        # Optional auth settings instead of oauth-client.json:
        # googleClientID = "YOUR_DESKTOP_CLIENT_ID"
        # googleClientSecret = "YOUR_DESKTOP_CLIENT_SECRET"
```

The JSON version should be considered deprecated, and new options will not be added
to it.  At some later date, it may be removed entirely.  Migrating to TOML is
recommended, not least because it's a much cleaner file format that supports handy
features like "comments" and "trailing commas in arrays" and "not needing to be wrapped
in braces and having a comma after every field".

## Known Issues

- Occasionally the shutdown is not as clean as it should be.
- Something seems to cause an occasional crash.
- If the blink(1) becomes disconnected, sometimes the program crashes instead of failing
    gracefully.

## Troubleshooting

Useful auth helpers:

- `./calblink -auth login` forces a browser-based login and stores a fresh token.
- `./calblink -auth logout` removes the cached token.
- `./calblink -auth status` shows whether a cached token exists and whether it
    includes a refresh token.
- `./calblink -list_calendars` authenticates if needed and prints the calendar
    names and IDs available to the signed-in account.

## Development

This repo includes a `pre-commit` configuration for lightweight hygiene and Go
checks similar to the setup used in other local projects.

1. Install `pre-commit`.
2. Install `golangci-lint` if you want the full Go lint hook enabled.
3. Install the hooks:

   ```bash
   pre-commit install
   ```

4. Run everything on demand:

   ```bash
   pre-commit run --all-files
   ```

The Go hooks run `gofmt`, `go mod tidy`, `go vet ./...`, `go test ./...`, and
`golangci-lint run --fix`.

## Releases

GitHub Actions publishes release archives automatically when you push a tag that
matches Semantic Versioning 2.0.0 with a leading `v`, for example:

```bash
git tag v1.2.3
git push origin v1.2.3
```

Pre-release tags such as `v1.2.3-rc.1` are supported and will be marked as
pre-releases automatically.

- If the blink(1) is flashing magenta, this means it was unable to connect to
    or authenticate to the Google Calendar server. If your network is okay, run
    `./calblink -auth status`. If needed, run `./calblink -auth login` to
    reauthorize, or `./calblink -auth logout` followed by
    `./calblink -auth login` to start over cleanly.
- If an error message about "no required module provides package..." comes up after
    updating calblink, run the following to update all needed modules:

    ```bash
    go get -u
    go mod tidy
    ```

- If building with `-tags blink1` fails, the most common issue is that CGO
    native build tools are missing. On macOS, install the Xcode Command Line
    Tools with `xcode-select --install` and then rebuild.

- If you need to inspect blink(1) device detection and open behavior, run
    `./calblink -list_blink1_devices`. This prints the HID enumeration details
    and whether calblink can actually open each matching device.

- Sending a SIGQUIT will turn on debug mode while the app is running.  By
    default on Unix-based systems, this is sent by hitting Ctrl-\\ (backslash).
    There is currently no way to turn debug mode off once it is set.
- If attempting to run gives an error about 'invalid\_grant' and 'Bad Request',
    your cached refresh token may have been revoked or your OAuth app may still
    be in testing mode. Re-run `./calblink -auth login` after checking your
    Google Cloud OAuth configuration.

## Legal

- Calblink is not an official Google product.
- Calblink is licensed under the Apache 2 license; see the LICENSE file for details.
- This repository is a derivative of the original Google calblink project; see
    the NOTICE file for attribution details.
- Calblink contains code from the [Google Calendar API
    Quickstart](https://developers.google.com/google-apps/calendar/quickstart/go)
    which is licensed under the Apache 2 license.
- Calblink uses the [Go service](https://github.com/kardianos/service/) library for
    managing service mode.
- All trademarks are the property of their respective holders.
