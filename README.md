# Captive Portal / WiFi Config for Raspberry Pi

A Go application for **Raspberry Pi Zero W 2** (or any Pi with WiFi) that:

- **When the Pi has no WiFi connectivity:** Starts an open access point (SSID: `ConfigureMe`). A captive DNS server resolves all client DNS lookups to the Pi, and HTTP is redirected to the config page, so when users connect they are prompted (or can open any page) to see the configuration form. No need to know the Pi’s IP. After submitting, the Pi reboots and connects to the configured network.
- **When the Pi is connected to WiFi:** Runs the same configuration portal on port 80 so you can change WiFi or other settings anytime.

It uses **NetworkManager** (`nmcli`) for WiFi and AP mode and is designed to be robust and work out of the box.

## Requirements

- Raspberry Pi (tested on Zero W 2) with WiFi
- NetworkManager installed and managing the WiFi interface
- Root or capabilities for: binding to ports 80 and 53 (captive DNS), running `nmcli`, `iptables`, and `systemctl reboot`

## Build

```bash
make build          # current platform
make build-pi       # Raspberry Pi Zero W 2 (armv7)
make build-arm      # generic Linux ARM (Pi 3/4, etc.)
```

Or with `go build` directly:

```bash
GOOS=linux GOARCH=arm GOARM=7 go build -o captive-portal .
```

## Install

1. Copy the binary to the Pi, e.g. `/usr/local/bin/captive-portal`.
2. Install the systemd unit:

   ```bash
   sudo cp captive-portal.service /etc/systemd/system/
   sudo systemctl daemon-reload
   sudo systemctl enable captive-portal
   sudo systemctl start captive-portal
   ```

3. **Run as root** (recommended for simplicity): edit the service and set `User=root`, and remove the `Capability*` lines. Then reload and start.

   Or run with capabilities (no root): ensure the binary has `CAP_NET_BIND_SERVICE`, `CAP_NET_ADMIN`, and `CAP_SYS_BOOT` (as in the service file).

4. Ensure **NetworkManager** is enabled and managing WiFi:

   ```bash
   sudo systemctl enable NetworkManager
   ```

## Behavior

- On startup the service waits 5 seconds for NetworkManager to connect to a known network.
- It then checks connectivity (HTTP request to `clients3.google.com/generate_204`). If that fails, it starts the open AP **CaptivePortalAP** (SSID: `ConfigureMe`), starts a captive DNS server on port 53 (so all client DNS queries resolve to the Pi at 10.42.0.1), and sets an iptables rule so that HTTP from clients (10.42.0.0/24) is redirected to the Pi. Any URL the client opens (e.g. captive portal detection or typing a domain) hits the config page, so users don’t need to know the Pi’s IP.
- Every 30 seconds it re-checks. If connectivity is restored, it stops the AP and removes the redirect.
- The config page is always available at `http://<pi-ip>/` (when on the same network or when connected to the open AP). Submitting the form with a new SSID/password adds a NetworkManager connection, saves config to `/etc/captive-portal/config.json`, and triggers a reboot so the Pi joins the new network.

## Environment (optional)

- **PORT** – HTTP port for the config portal (default: `80`). Use e.g. `8080` for testing without root.
- **CONFIG_PATH** – Config file path (default: `/etc/captive-portal/config.json`).

## Config file

- Path: `/etc/captive-portal/config.json` (or `CONFIG_PATH`)
- Contains `ssid`, `password`, and an `extra` map for future options. The HTML form can be extended with more fields that are stored under `extra`.

## Extending the config page

- **HTML:** Edit `internal/portal/templates/index.html`. The “Other options” section is intended for non-WiFi settings; add form fields with `name` values that are not `ssid` or `password`; they are stored in `config.extra` and can be read by your app.
- **Handling:** Form POST to `/configure` already saves all non-`ssid`/`password` fields into `config.extra`. Add logic in your code to read `cfg.GetExtra("key")` and use those values.

## Robustness

- Retries for `nmcli` when adding WiFi or starting the AP.
- WiFi interface is auto-detected via `nmcli device status`.
- Config load failure leaves the app running with empty config.
- Service has `Restart=always` so it recovers from crashes.

## License

MIT.
