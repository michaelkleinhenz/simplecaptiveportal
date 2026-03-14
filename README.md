# Captive Portal / WiFi Config for Raspberry Pi

A Go application for **Raspberry Pi** (Zero W 2, Pi 5, etc.) that:

- **When the Pi has no WiFi connectivity:** Starts an open access point. The AP SSID is **`<hostname>Configure`** (e.g. `living-roomConfigure`); if the hostname is empty it falls back to **`ConfigureMe`**. A captive DNS setup (and optional local DNS server) makes client DNS resolve to the Pi, and HTTP is redirected to the config page, so users who connect see the configuration form without needing the Pi’s IP. Saving WiFi does **not** reboot; use the **Reboot** button when you want to switch network.
- **When the Pi is connected to WiFi:** The same configuration portal runs on port 80 so you can change WiFi, Snapcast, hostname, or reboot/shutdown anytime.

It uses **NetworkManager** (`nmcli`) for WiFi and AP mode and is designed to work on current and older Raspberry Pi OS.

## Requirements

- Raspberry Pi with WiFi (tested on Zero W 2, Pi 5)
- **NetworkManager** installed and managing the WiFi interface
- Root (or appropriate capabilities) for: binding to ports 80 and 53, `nmcli`, `iptables`, `systemctl reboot`, `systemctl poweroff`, `hostnamectl`/`/etc/hostname`, writing `/etc/default/snapclient`

## Build

```bash
make build          # current platform
make build-pi       # Raspberry Pi Zero W 2 (armv7)
make build-arm      # generic Linux ARM (Pi 3/4, Pi 5, etc.)
make deps           # download Go modules (optional)
```

Or with `go build`:

```bash
GOOS=linux GOARCH=arm GOARM=7 go build -o captive-portal .   # Pi Zero W 2
GOOS=linux GOARCH=arm64 go build -o captive-portal .         # Pi 5
```

## Install

**Local install (on the Pi):**

```bash
sudo make install
```

This builds for Pi (armv7), installs the binary to `/usr/local/bin/captive-portal` and the systemd unit to `/etc/systemd/system/`, then runs `daemon-reload`, `enable`, and `restart`.

**Remote install (from your machine):**

```bash
make install-remote REMOTE=pi@192.168.1.100
```

This builds, copies the binary and service file to the Pi over SSH, installs them, and restarts the service.

**Manual install:**

1. Copy the binary to the Pi (e.g. `/usr/local/bin/captive-portal`) and the unit file to `/etc/systemd/system/captive-portal.service`.
2. Run as root (the service file uses `User=root`).
3. Enable and start: `sudo systemctl daemon-reload && sudo systemctl enable captive-portal && sudo systemctl start captive-portal`.
4. Ensure NetworkManager is enabled: `sudo systemctl enable NetworkManager`.

## Web UI

- **WiFi:** Scan networks, choose SSID, enter password. **Save WiFi** adds the connection in NetworkManager (no reboot). Use **Reboot** when you want to switch to the new network.
- **Snapcast Server:** IP or hostname of the Snapcast server. Saved to `/etc/default/snapclient` as `SNAPCLIENT_OPTS="-h <ip>"`; the snapclient service is restarted.
- **Snapcast Client Name:** Device hostname (shown in Snapcast). Written to `/etc/hostname` and applied with `hostnamectl` (or fallback). Snapclient is restarted so it picks up the new name.
- **Reboot** and **Shutdown** buttons: with confirmation. Reboot is needed after changing WiFi if you want to join the new network.

The page title and heading show the current hostname when set (e.g. `living-room – Device configuration`).

## Behavior

- On startup the service waits 5 seconds for NetworkManager to connect.
- It checks connectivity (HTTP to `clients3.google.com/generate_204`). If that fails, it starts the open AP **CaptivePortalAP** with SSID **`<hostname>Configure`** (or `ConfigureMe`), sets up iptables to redirect HTTP from clients to the Pi, and optionally starts a captive DNS server. To avoid “IP configuration error” on older Raspberry Pi OS, the AP is brought up first and the app waits **3 seconds** before binding to port 53, so NetworkManager’s DHCP/DNS can start first. A dnsmasq snippet in `/etc/NetworkManager/dnsmasq-shared.d/` is also written so that where supported, all DNS resolves to the gateway.
- Every 30 seconds it re-checks. If connectivity is restored, it stops the AP and cleans up.
- The config page is always available at `http://<pi-ip>/` (same network or when connected to the config AP). Changes are applied on save; use **Reboot** when you want to apply a new WiFi network.

## Config and files

- **`/etc/captive-portal/config.json`** (or `CONFIG_PATH`): JSON with `ssid`, `password`, and `extra` for other options.
- **`/etc/default/snapclient`**: Single line `SNAPCLIENT_OPTS="-h <ip>"` for the Snapcast server; written when you save **Snapcast Server**.
- **`/etc/hostname`**: Written when you save **Snapcast Client Name** so the hostname persists across reboots.

## Environment (optional)

- **PORT** – HTTP port for the portal (default `80`). Use e.g. `8080` for testing without root.
- **CONFIG_PATH** – Config file path (default `/etc/captive-portal/config.json`).

## Robustness

- Retries for `nmcli` when adding WiFi or starting the AP.
- WiFi interface is auto-detected via `nmcli device status`.
- Config load failure leaves the app running with empty config.
- Service has `Restart=always`.
- Hostname is always written to `/etc/hostname` for persistence; then applied live with `hostnamectl` or a fallback.
- On older Raspi OS, a delay before starting the captive DNS avoids stealing port 53 from NetworkManager so DHCP works.

## License

MIT.
