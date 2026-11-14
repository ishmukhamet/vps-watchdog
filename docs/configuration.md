# Configuration

Config file: `/etc/vps-watchdog/config.yaml`. The installer generates it; a
full example is in
[configs/config.example.yaml](../configs/config.example.yaml).

```bash
vps-watchdog validate-config --config /etc/vps-watchdog/config.yaml  # offline check
vps-watchdog test-notify --config /etc/vps-watchdog/config.yaml      # end-to-end delivery
```

`validate-config` checks structure, unknown keys, and value ranges; it does not
contact Telegram. Restart the service after editing the config.

## Format

The parser accepts a deliberately small YAML subset to avoid a YAML dependency:
nested `key: value` mappings indented by two spaces, scalar values, and `#`
comments. Anchors, lists, and multi-line strings are not supported. Unknown keys
are errors.

## Keys

| Key | Default | Meaning |
| --- | --- | --- |
| `host_id` | hostname | Name shown in every alert. |
| `telegram.token_file` | `/etc/vps-watchdog/telegram.token` | File with the bot token. |
| `telegram.chat_id_file` | `/etc/vps-watchdog/telegram.chat_id` | File with the target chat id. |
| `login.enabled` | `true` | Login alerts. |
| `sudo.enabled` | `true` | Sudo alerts. |
| `load.enabled` | `true` | Load alerts. |
| `load.sample_interval_sec` | `15` | How often `/proc` is sampled. |
| `load.cooldown_sec` | `900` | Minimum time between repeated alerts for an ongoing condition. |
| `load.cpu_busy_percent` | `90` | CPU busy share that counts as a breach. |
| `load.cpu_duration_sec` | `180` | How long CPU must stay breached before alerting. |
| `load.mem_available_percent` | `10` | Available memory share below which memory counts as breached. |
| `load.mem_duration_sec` | `180` | How long memory must stay breached. |
| `load.psi_io_some_avg60` | `20` | PSI I/O `some avg60` that counts as a breach. Ignored on hosts without PSI. |
| `load.io_duration_sec` | `180` | How long I/O pressure must stay breached. |
| `disk.enabled` | `true` | Low disk space and inode alerts. |
| `disk.paths` | `"/"` | Comma-separated absolute paths whose filesystems are watched, at most 16. Paths under `/home`, `/root`, `/run/user`, `/tmp`, and `/var/tmp` are rejected: the systemd sandbox hides them from the service. |
| `disk.sample_interval_sec` | `60` | How often free space is measured. |
| `disk.cooldown_sec` | `21600` | Minimum time between repeated alerts for a filesystem that stays low. |
| `disk.duration_sec` | `300` | How long free space must stay low before alerting; `0` alerts on the first sample. |
| `disk.free_percent` | `10` | Free space share, as `df` reports it for unprivileged users, below which a filesystem counts as low. |
| `disk.inodes_free_percent` | `10` | Free inode share below which a filesystem counts as low; `0` disables inode alerts. |
| `reboot.enabled` | `true` | Report reboots with the previous run's end and approximate downtime. |
| `liveness.enabled` | `false` | Send pings to an external dead man's switch. |
| `liveness.ping_url_file` | `/etc/vps-watchdog/liveness.url` | File with the HTTPS ping URL. |
| `liveness.interval_sec` | `60` | Seconds between pings; at least `10`. |

Load and disk alerts fire only after a condition holds for its full duration,
repeat no more often than the cooldown, and are followed by a recovery alert.
Each alert shows the measured value and the threshold; disk alerts also show
absolute free space.

## Liveness pings

Liveness pings let an external service notice that the host went silent:
powered off, cut off from the network, or with the agent stopped. See
[ADR 0008](adr/0008-external-liveness-monitoring.md).

With [Healthchecks](https://healthchecks.io) (hosted or self-hosted):

1. Create a check. Set its period to `liveness.interval_sec` and its grace time
   to how long a silence you accept, for example 5–10 minutes to ride out
   reboots and short network outages.
2. Put the check's ping URL into the URL file and restrict it:

   ```bash
   sudo sh -c 'cat > /etc/vps-watchdog/liveness.url'   # paste the URL, then Ctrl-D
   sudo chown root:vpswatchdog /etc/vps-watchdog/liveness.url
   sudo chmod 0640 /etc/vps-watchdog/liveness.url
   ```

3. Set `liveness.enabled: true`, run `validate-config`, and restart the service.

`validate-config` checks that the URL file is readable and contains an HTTPS
URL, without printing it. The agent pings only while every enabled journal
watcher is running, so a watcher that cannot read the journal also lets the
check go down. Other services that accept a plain `GET` to a fixed URL work the
same way.

## Credentials

The token, chat id, and liveness ping URL live in separate files, never in the
config itself. Keep them owned by `root:vpswatchdog` with mode `0640`; the installer enforces this on
every run. See [SECURITY.md](SECURITY.md#principles) for the rules around
secrets.
