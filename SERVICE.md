# Running calblink as a service

## What does this do?

Calblink now supports a mode where it runs as a service. This means that it is managed
by your operating system instead of needing to manually run it. This service can be
turned on and survive reboots.

## What operating systems does this mode support?

It supports per-user launchd on macOS and per-user systemd on Linux. Static service
files are provided in `files/macos` and `files/linux`.

## What potential problems are there for this mode?

calblink currently doesn't cope well with not having a blink(1) installed when it is run.
It will exit after enough failures to control the blink(1), if it doesn't segfault first.
This mode works best for cases where a machine has a blink(1) set up at all times.
Alternately, if you have a way of controlling launch daemons based on USB events
(EventScripts or similar on macOS) you can use that to only run calblink when there
is a blink(1) plugged in.

If you don't disable the launch daemon when there isn't a blink(1) plugged in, calblink
will crash and be automatically restarted every ten seconds or so.

## How do I set this up on macOS?

These instructions use the checked-in launchd file.

1. Install calblink like you normally would, then make sure your configuration
   is set up the way you want. The provided launch agent expects `calblink` to
   be available in `/usr/local/bin` or `/opt/homebrew/bin`.
2. Install and load the launch agent:

   ```bash
   install -m 0644 files/macos/com.spkane.calblink.plist ~/Library/LaunchAgents/
   launchctl bootstrap "gui/$(id -u)" ~/Library/LaunchAgents/com.spkane.calblink.plist
   launchctl enable "gui/$(id -u)/com.spkane.calblink"
   launchctl kickstart "gui/$(id -u)/com.spkane.calblink"
   ```

3. Control it with `launchctl`:

   ```bash
   launchctl print "gui/$(id -u)/com.spkane.calblink"
   launchctl bootout "gui/$(id -u)" ~/Library/LaunchAgents/com.spkane.calblink.plist
   ```

4. Log messages go to `/tmp/com.spkane.calblink.out.log` and
   `/tmp/com.spkane.calblink.err.log`.

## How do I set this up on Linux?

These instructions assume a user-level systemd service.

1. Install calblink like you normally would, then make sure your configuration
   is set up the way you want. The provided unit expects `calblink` to be
   installed at `~/.local/bin/calblink`.
2. Install and start the user service:

   ```bash
   install -D -m 0644 files/linux/calblink.service ~/.config/systemd/user/calblink.service
   systemctl --user daemon-reload
   systemctl --user enable --now calblink.service
   ```

3. Control and inspect it with systemd:

   ```bash
   systemctl --user status calblink.service
   journalctl --user-unit calblink.service
   systemctl --user restart calblink.service
   ```

The older built-in service installer is still available with
`./calblink -runAsService -service install`, but the checked-in service files are
preferred because they are reviewable and easier to manage with normal OS tools.
