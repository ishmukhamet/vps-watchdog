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

Load alerts fire only after a condition holds for its full duration, repeat no
more often than the cooldown, and are followed by a recovery alert.

## Credentials

The token and chat id live in separate files, never in the config itself. Keep
them owned by `root:vpswatchdog` with mode `0640`; the installer enforces this on
every run. See [SECURITY.md](SECURITY.md#principles) for the rules around
secrets.
