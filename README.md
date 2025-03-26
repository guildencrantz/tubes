# Tubes

`[T]he Internet is not something that you just dump something on. It's not a
big truck. It's a series of tubes.` - Sen. Ted Stevens (R-Alaska), June 28, 2006

## Goals

Dynamically create tunnels (over SSH using `ssh-agent` for authentication).
`tubes` quietly sit in the background listening for connections to local ports,
and when one is made forwarding that connection somewhere else, creating any
necessary connections dynamically.

## Installation

`go get github.com/guildencrantz/tubes`

## Configuration

A config file is expected at `~/.config/tubes.{json,yaml,hcl}`.

```
ssh:
  - Name:     "foo"
    User:     "me"
    Hostname: "foo.test.com"
    Tunnels:
      - Name: "SOCKS"
        Port: 2600
  - Name: "bar"
    User: "jayne"
    Hostname: "test.com"
    Port: 2222,
    Tunnels:
      - Name:       "Dev DB"
        LocalBind:  "127.0.0.2"
        Port:       5431
        RemoteBind: "db.dev.com"
        RemotePort: 5432
      - Name:       "Prod DB"
        Port:       5434
        RemoteBind: "db.prod.com"
        RemotePort: 5432
        Disabled:   true
```

This config demonstrates three ports forwarding over two different SSH connections.

The first SSH connection has a single tunnel associated with it: It'll forward
port local machine's `127.0.0.1:2600` port to `me@foo.test.com:22`'s
`127.0.0.1:2600`. When `tubes` start it'll automatically start listening on `5431`,
and will connect to `me@foo.test.com:22` when it first receives a local connection.

The second SSH connection is more complicated:

- Both tunnels will go over a single SSH connection to `jayne@test.com:2222`.
- Tunnel 1 (`Dev DB`) forwards the local machine's `127.0.0.2:5431` to
  `db.dev.com:5432`. The local listener starts automatically and the SSH
  connection is created on first usage.
- Tunnel 2 (`Prod DB`) forwards the local machine's `127.0.0.1:5434` to
  `db.prod.com:5432`. The local listener _does not_ start automatically and no
  SSH connection will be made if a connection is made to local `127.0.0.1:5434`.

## Usage

The config is slurped in at startup and any tunnels that aren't disabled will
have their local listeners automatically started. When started a system tray
item named `tubes` should appear. Any tunnels that are actively listening will
have a check by them. You can disable a listening tunnel by clicking its name.
Likewise you can enable a disabled tunnel by clicking its name.

Similarly with the SSH connections themselves if there's a check next to the
name in the list a connection is live, and if it's unchecked it's dormant. You
can force a connection to start or close by clicking on it. If you close an SSH
connection but haven't disabled its tunnels a new connection will be recreated
if the tunnel is used.

You can, optionally, force all the connections to be closed and all the tunnels
to be reset to their default state by clicking `Restart`.

To quit `tubes` use the `Quit` menu item, or just kill the process (which isn't
actually daemonized as of this writing).

### As a service

You can run `tubes` as `launchd` service by creating a
`~/Library/LaunchAgents/tubes.plist`, and then enabling it
(`launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/tubes.plist`). My
file (you'll need to fix the paths for you):

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
 <key>EnvironmentVariables</key>
 <dict>
  <key>SSH_AUTH_SOCK</key>
  <string>/Users/mhenkel/.gnupg/S.gpg-agent.ssh</string>
 </dict>
 <key>KeepAlive</key>
 <dict>
  <key>Crashed</key>
  <true/>
  <key>SuccessfulExit</key>
  <false/>
 </dict>
 <key>Label</key>
 <string>tubes</string>
 <key>ProgramArguments</key>
 <array>
  <string>/Users/mhenkel/workspace/go/bin/tubes</string>
 </array>
 <key>RunAtLoad</key>
 <true/>
 <key>StandardErrorPath</key>
 <string>/tmp/tubes.stderr</string>
 <key>StandardOutPath</key>
 <string>/tmp/tubes.stdout</string>
</dict>
</plist>
```

# Warnings

This is a weekend project with a few hours total time in it, and as such less
than that in testing. Basic testing looks good, but I know there are issues
(there's minimal error handling, so you're pretty much guaranteed to get odd
behavior and hangs under any edge case--including switching networks).
