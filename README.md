# Scaffold Command Quick Reference

## Restart Services (systemd user units)

```bash
cd /home/mikekey/Builds/scaffold/daemon
systemctl --user daemon-reload
systemctl --user restart scaffold-signal-cli.service
systemctl --user restart scaffold-daemon.service
```

Check status:

```bash
systemctl --user --no-pager status scaffold-signal-cli.service
systemctl --user --no-pager status scaffold-daemon.service
```

## Health Check

```bash
curl -sS http://127.0.0.1:46873/api/health
```

Expected:

```json
{"status":"ok"}
```

## Logs

Tail daemon:

```bash
journalctl --user -u scaffold-daemon.service -f
```

Tail signal-cli:

```bash
journalctl --user -u scaffold-signal-cli.service -f
```

## Rebuild Daemon Binary

Use this after Go code changes (the service runs `bin/scaffold-daemon`):

```bash
cd /home/mikekey/Builds/scaffold/daemon
go build -o bin/scaffold-daemon .
systemctl --user restart scaffold-daemon.service
```

## Frontend Dev

```bash
cd /home/mikekey/Builds/scaffold/app
npm run dev
```

## Test Commands

Backend:

```bash
cd /home/mikekey/Builds/scaffold/daemon
go test ./...
```

Frontend build smoke check:

```bash
cd /home/mikekey/Builds/scaffold/app
npm run build
```

## Auth Env Reminders

In `/home/mikekey/Builds/scaffold/daemon/.env` set:

- `APP_USERNAME`
- `APP_PASSWORD_HASH`
- `COOKIE_SECURE=true` for HTTPS deployments (`false` only for local HTTP dev)

Generation helper for `APP_PASSWORD_HASH` is documented in `/home/mikekey/Builds/scaffold/daemon/.env.sample`.
