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
sudo journalctl --no-pager -n 20 _COMM=sudo
sudo journalctl --no-pager -n 20 _EXE=/usr/bin/sudo
sudo journalctl --no-pager -n 20 SYSLOG_IDENTIFIER=sudo
```

The agent follows sudo entries across journals visible to the service runtime
user. If root can see entries but the service cannot, check the rendered unit for
`SupplementaryGroups=systemd-journal adm` and inspect service logs for
`journalctl stderr` permission errors.

### Old events after reinstall

The normal first-start behavior is to begin watching at the current time when no
cursor exists. If old events appear after manual state restoration, check whether
`/var/lib/vps-watchdog/login.cursor` or `/var/lib/vps-watchdog/sudo.cursor` came
from another host or an older journal namespace.
