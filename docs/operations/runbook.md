# Runbook

## Check service status

```bash
sudo systemctl status vps-watchdog.service
```

## Read logs

```bash
sudo journalctl -u vps-watchdog.service -n 200 --no-pager
```

## Validate config

```bash
sudo /usr/local/bin/vps-watchdog validate-config --config /etc/vps-watchdog/config.yaml
```

## Send test Telegram alert

```bash
sudo /usr/local/bin/vps-watchdog test-notify --config /etc/vps-watchdog/config.yaml
```

## Check PAM hook events

Trigger a login, then inspect:

```bash
sudo journalctl --identifier=vps-watchdog-login -n 20 --no-pager
```

## Check sudo events

```bash
sudo journalctl --identifier=sudo -n 20 --no-pager
```

## Restart agent

```bash
sudo systemctl restart vps-watchdog.service
```

## Common failures

### No Telegram message

Check:

- token file exists;
- chat id file exists;
- outbound HTTPS to Telegram is allowed;
- `test-notify` output;
- service logs do not show HTTP errors.

### No login alerts

Check:

- PAM line exists in `/etc/pam.d/sshd` or `/etc/pam.d/login`;
- `/usr/local/libexec/vps-watchdog/pam-login-event` is executable;
- `logger` exists;
- `od` and `tr` exist for PAM value encoding;
- journal contains `vps-watchdog-login` entries.

### No sudo alerts

Check whether sudo logs to journald on the host:

```bash
sudo journalctl --identifier=sudo -n 20 --no-pager
```

If no entries appear, distro-specific sudo/syslog configuration may be needed.

### Old events after reinstall

The normal first-start behavior is to begin watching at the current time when no
cursor exists. If old events appear after manual state restoration, check whether
`/var/lib/vps-watchdog/login.cursor` or `/var/lib/vps-watchdog/sudo.cursor` came
from another host or an older journal namespace.
