# RVMM - GitHub Actions Runner VM Manager

A tool for managing macOS virtual machines as GitHub Actions self-hosted runners using Tart on Apple Silicon.

## Features

- **VM Management**: Build, configure, and manage Tart VMs for GitHub Actions runners
- **Runner Daemon**: Automatically start and manage GitHub Actions runners
- **Log Monitoring**: Send VM logs to PostHog for centralized monitoring across multiple machines
- **Interactive TUI**: User-friendly terminal interface for all operations
- **Headless Mode**: Run in background via LaunchAgent/LaunchDaemon

## Installation

### Prerequisites

- macOS on Apple Silicon (M1/M2/M3)
- [Homebrew](https://brew.sh/)
- Required tools (can be installed via setup command):
  - `tart` - VM management
  - `sshpass` - SSH automation
  - `wget` - File downloads
  - `packer` - VM image building

### Build from Source

```bash
git clone <repository-url>
cd macos-github-action-vm
go build -o rvmm main.go
```

## Configuration

Create a `rvmm.yaml` configuration file (see `assets/config.yaml.example` for a template):

```yaml
github:
  api_token: "ghp_xxxxxxxxxxxxxxxxxxxx"
  registration_endpoint: "https://api.github.com/orgs/YOUR_ORG/actions/runners/registration-token"
  runner_url: "https://github.com/YOUR_ORG"
  runner_name: "runner"
  runner_labels:
    - self-hosted
    - arm64
    - macOS
  runner_group: ""

vm:
  username: "admin"
  password: "admin"

registry:
  url: ""
  image_name: "runner:latest"
  username: ""
  password: ""

options:
  truncate_size: ""
  log_file: "runner.log"
  max_concurrent_runners: 1
  shutdown_flag_file: ".shutdown"
  working_directory: "/Users/admin/vm"

daemon:
  label: "com.mirego.ekiden"
  plist_path: "/Users/admin/Library/LaunchAgents/com.mirego.ekiden.plist"
  user: "admin"

posthog:
  enabled: false
  api_key: "phc_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
  host: "https://app.posthog.com"
  machine_label: "machine-1"
```

## Usage

### Interactive Mode (TUI)

Run without arguments to start the interactive terminal UI:

```bash
./rvmm
```

The TUI provides menus for:

- **Setup**: Install required dependencies
- **Build**: Build VM image from IPSW
- **Config**: Edit configuration
- **Run**: Start runners interactively
- **Images**: List local VM images
- **Daemon**: Install/manage runner daemon
- **Monitor Daemon**: Install/manage log monitoring daemon
- **View Logs**: Tail log files

### Headless Mode

#### Run Runner

Start the GitHub Actions runner in foreground:

```bash
./rvmm run -config rvmm.yaml
```

#### Monitor Logs

Start log monitoring to send logs to PostHog:

```bash
./rvmm monitor -config rvmm.yaml
```

This monitors `/Users/qiweili/rvmm/stdout` and `/Users/qiweili/rvmm/stderr` and sends each new log line to PostHog with the configured machine label.

### Daemon Management

#### Runner Daemon

Install as a LaunchAgent/LaunchDaemon to run automatically:

**Via TUI:**

1. Start `./rvmm`
2. Select "Daemon" → "Install daemon"

**Via Command Line:**

- The daemon can be controlled using standard `launchctl` commands after installation

#### Monitor Daemon

Install log monitoring as a background service:

**Via TUI:**

1. Start `./rvmm`
2. Select "Monitor Daemon" → "Install monitor daemon"

**Requirements:**

- PostHog must be enabled in `rvmm.yaml`
- Valid PostHog API key and machine label configured

The monitor daemon will:

- Start automatically on user login
- Continuously monitor log files
- Send new log lines to PostHog with machine label for differentiation
- Restart automatically if it crashes

**Check Status:**

```bash
launchctl print gui/$(id -u)/com.mirego.ekiden.monitor
```

**View Monitor Logs:**

```bash
tail -f /Users/qiweili/rvmm/monitor_stderr.log
```

## PostHog Log Monitoring

The log monitoring feature sends VM logs to PostHog for centralized analysis across multiple machines.

### Setup

1. **Get PostHog API Key:**
   - Sign up at [posthog.com](https://posthog.com) or use your self-hosted instance
   - Get your Project API Key from Settings → Project

2. **Configure in `rvmm.yaml`:**

   ```yaml
   posthog:
     enabled: true
     api_key: "phc_your_actual_api_key"
     host: "https://app.posthog.com"
     machine_label: "mac-studio-1" # Unique label for this machine
   ```

3. **Install Monitor Daemon:**
   ```bash
   ./rvmm  # Open TUI
   # Select "Monitor Daemon" → "Install monitor daemon"
   ```

### What Gets Monitored

- Files monitored:
  - `/Users/qiweili/rvmm/stdout`
  - `/Users/qiweili/rvmm/stderr`

- Each log line is sent as a PostHog event with properties:
  - `machine_label`: Your configured label (e.g., "mac-studio-1")
  - `log_type`: "stdout" or "stderr"
  - `log_line`: The actual log content
  - `timestamp`: When the log was captured

### Querying in PostHog

In PostHog, you can:

- Filter events by `machine_label` to see logs from specific machines
- Filter by `log_type` to see only stdout or stderr
- Search `log_line` content for specific errors or patterns
- Create dashboards to monitor error rates across machines

## Architecture

- **internal/config**: Configuration management with Viper
- **internal/runner**: GitHub Actions runner logic and VM management
- **internal/daemon**: LaunchAgent/LaunchDaemon installation and management
- **internal/monitor**: Log file monitoring with tail-follow logic
- **internal/posthog**: PostHog API client for log event capture
- **internal/tui**: Bubble Tea terminal UI
- **assets**: Embedded templates and example configs
- **guest**: VM image building scripts (Packer)

## Daemon Files

After installation, daemons create the following files:

### Runner Daemon

- Plist: As configured in `daemon.plist_path`
- Logs: `${working_directory}/stdout`, `${working_directory}/stderr`

### Monitor Daemon

- Plist: `${daemon.plist_path}` with `.monitor` suffix
- Logs: `${working_directory}/monitor_stdout.log`, `${working_directory}/monitor_stderr.log`

## Troubleshooting

### Runner Daemon Issues

Check daemon status:

```bash
launchctl print gui/$(id -u)/com.mirego.ekiden
```

View logs:

```bash
tail -f /Users/qiweili/rvmm/stderr
```

### Monitor Daemon Issues

Check if monitor is running:

```bash
launchctl print gui/$(id -u)/com.mirego.ekiden.monitor
```

View monitor logs:

```bash
tail -f /Users/qiweili/rvmm/monitor_stderr.log
```

Common issues:

- **PostHog not enabled**: Ensure `posthog.enabled: true` in config
- **Missing API key**: Add valid PostHog API key to config
- **Invalid machine_label**: Set a unique label for each machine
- **Log files not found**: Ensure runner daemon is running and creating logs

### Uninstall

**Via TUI:**

1. Start `./rvmm`
2. Select "Daemon" → "Uninstall daemon" (for runner)
3. Select "Monitor Daemon" → "Uninstall monitor daemon" (for monitoring)

**Manual Cleanup:**

```bash
# Uninstall runner daemon
launchctl bootout gui/$(id -u)/com.mirego.ekiden
rm ~/Library/LaunchAgents/com.mirego.ekiden.plist

# Uninstall monitor daemon
launchctl bootout gui/$(id -u)/com.mirego.ekiden.monitor
rm ~/Library/LaunchAgents/com.mirego.ekiden.monitor.plist
```

## License

[Your License Here]

## Contributing

[Contributing Guidelines Here]
