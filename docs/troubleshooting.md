# Runbook

Start every investigation with the service state and its own logs; the agent
reports watcher, permission, and delivery errors there.

```bash
sudo systemctl status vps-watchdog.service
sudo journalctl -u vps-watchdog.service -n 200 --no-pager
```

Then narrow down by stage: **config → delivery → event source → watcher**.

## No alerts at all

1. `sudo vps-watchdog validate-config --config /etc/vps-watchdog/config.yaml`
2. `sudo vps-watchdog test-notify --config /etc/vps-watchdog/config.yaml`

If `test-notify` fails: check that both credential files are non-empty and that
outbound HTTPS to `api.telegram.org` is allowed. The error text is sanitized and
safe to share once host-specific details are removed.

If `test-notify` works but alerts do not arrive, look in the service log for
dropped-event or delivery errors.

## No login alerts

Is the hook writing events? Log in again, then:

```bash
sudo journalctl --identifier=vps-watchdog-login -n 20 --no-pager
```

- **No entries:** the PAM line is missing from `/etc/pam.d/sshd` or
  `/etc/pam.d/login`, the hook in `/usr/local/libexec/vps-watchdog/` is not
  executable, or `logger`, `od`, or `tr` is missing. Rerunning the installer
  repairs the first two.
- **Entries exist:** the problem is in the watcher; see
  [Watcher cannot read the journal](#watcher-cannot-read-the-journal).

## No sudo alerts

Find out where sudo logs on this host:

```bash
sudo journalctl --no-pager -n 20 _COMM=sudo
sudo journalctl --no-pager -n 20 SYSLOG_IDENTIFIER=sudo
```

- **No entries:** sudo is not logging to journald (check sudo and syslog
  configuration); the agent cannot see it.
- **Entries exist:** the watcher cannot read them; see below.

## Watcher cannot read the journal

Root sees entries, but the service does not. The service log usually contains
`journalctl stderr` with a permission error. The runtime user needs a
journal-reading group: check that `SupplementaryGroups=` in
`/etc/systemd/system/vps-watchdog.service` lists `systemd-journal` and/or `adm`.
The installer adds whichever groups exist on the host, so rerunning it after
adding a group fixes the unit.

## Old events after restoring state

Watchers resume from cursors stored in `/var/lib/vps-watchdog`. A cursor copied
from another host or an old journal can cause skipped or replayed events. Stop
the service, delete the `*.cursor` files, and start it again: watching resumes
from the current time
([ADR 0007](adr/0007-journal-cursor-start-at-now.md)).

A cursor that is malformed or that journalctl cannot use is moved aside
automatically: the service log shows `journal cursor rejected by journalctl` or
`has an invalid format`, and the old value is kept in `*.cursor.rejected` for
inspection. Watching resumes from the current time, so
events since the last saved position are not alerted.

## Liveness check is down

The dead man's switch reports the host as down, but the host is up.

1. Look for `liveness` in the service log. `liveness pings disabled` names the
   problem with the URL file; `liveness ping failed` shows the sanitized
   network or HTTP error; `liveness ping skipped: a journal watcher is not
   running` means a watcher keeps failing, see
   [Watcher cannot read the journal](#watcher-cannot-read-the-journal).
2. `sudo vps-watchdog validate-config --config /etc/vps-watchdog/config.yaml`
   checks the URL file without printing the URL.
3. Check that outbound HTTPS to the ping service is allowed.

## Reboot reported but none was planned

The alert shows whether the previous run stopped cleanly and the approximate
downtime. A clean stop means the system shut down normally; no clean stop means
a forced reset, power loss, or kernel crash. Compare the time with the provider's
maintenance notices and the console or panel audit log. `journalctl --list-boots`
shows the boots journald retained.

## No disk alerts for a mount point

- `validate-config` rejects paths that the systemd sandbox hides from the
  service (`/home`, `/root`, `/run/user`, `/tmp`, `/var/tmp`).
- `disk check <path>: ...` in the service log means the path cannot be
  measured, for example because it does not exist.
- Paths on the same filesystem are reported once, under the first path listed.
  A mount point whose filesystem is not mounted is an ordinary directory on its
  parent filesystem, so it is silently counted as part of the parent.
- Btrfs and some other filesystems report no inode counts; they get space alerts
  only.
