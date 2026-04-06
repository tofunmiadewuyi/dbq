# dbq

A CLI tool for scheduling and running PostgreSQL backups, with support for local directory storage or cloud storage (AWS S3 / Cloudflare R2).

## Features

- Interactive terminal UI for creating and managing backup jobs
- Backup to a local directory or upload directly to S3 / R2
- SSH support — dump a remote database over an SSH connection
- Server-side mode — dump and upload entirely on the server, bypassing your home internet
- Automatic compression with pg_dump's custom format (`-Fc`)
- Schedule backups with systemd timers (no cron needed)
- Per-job log files with timing for every run and test

## Requirements

- Go 1.21+
- `pg_dump` installed wherever the database lives (locally or on the SSH host)
- For scheduled backups: systemd (Linux)
- For server-side uploads: `curl` on the remote host (installed by default on most Linux servers)

## Installation

```bash
git clone https://github.com/tofunmiadewuyi/dbq
cd dbq
go build -o dbq ./cmd
```

Move the binary somewhere on your `$PATH`:

```bash
mv dbq /usr/local/bin/
```

## Usage

### Interactive mode

```bash
dbq
```

Launches the interactive menu. From here you can create jobs, run them manually, test connections, and schedule/unschedule them.

### Run a job directly (used by systemd)

```bash
dbq run <job-id>
```

Runs the backup for the given job ID non-interactively. This is what the generated systemd service calls.

## Job configuration

Jobs are stored as TOML files in `~/.config/dbq/jobs/` (or `/etc/dbq/jobs/` when running as root). You can create them through the interactive UI or edit them directly.

### Example — local database, upload to S3

```toml
name = "my-app"
id = "my-app"
storage_type = "cloud"
frequency = "0 2 * * *"

[database]
  name = "mydb"
  type = "postgres"
  host = "localhost"
  port = "5432"
  username = "postgres"
  password = "secret"

[storage]
  provider = "AWS (S3)"
  bucket = "my-backups"
  region = "eu-west-1"
  access_key = "AKIAIOSFODNN7EXAMPLE"
  secret_key = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
```

### Example — remote database over SSH, upload to R2

```toml
name = "prod"
id = "prod"
storage_type = "cloud"
frequency = "0 3 * * *"

[database]
  name = "proddb"
  type = "postgres"
  host = "localhost"
  port = "5432"
  username = "archive"
  password = "secret"

  [database.ssh]
    required = true
    sshhost = "1.2.3.4"
    sshuser = "ubuntu"
    sshkey = "/home/you/.ssh/id_rsa"
    sshport = 22
    useserver = false   # set to true to dump and upload on the server (see below)

[storage]
  provider = "Cloudflare R2"
  bucket = "my-backups"
  endpoint = "https://<account-id>.r2.cloudflarestorage.com"
  access_key = "..."
  secret_key = "..."
```

### `useserver` mode

When `useserver = true`, dbq does not stream the dump over your SSH connection. Instead:

1. `pg_dump` runs on the server and writes to `/var/tmp/dbq/` on that host
2. A presigned PUT URL is generated on your machine (no credentials leave it)
3. The server uploads the file directly to S3/R2 via `curl`
4. The temp file is deleted from the server

This is useful when your SSH connection is slow (e.g. a small EC2 instance on a throttled tier) but the server has fast outbound internet to AWS/Cloudflare.

Requirements: `curl` on the server, outbound HTTPS (port 443) allowed — both are true by default on most cloud hosts.

## Scheduling

From the job management menu, select **Schedule** to install a systemd timer for the job. This creates:

- `~/.config/systemd/user/dbq-<id>.service`
- `~/.config/systemd/user/dbq-<id>.timer`

The timer uses `Persistent=true`, meaning a missed run (e.g. the machine was off) will fire as soon as it comes back online.

Select **Unschedule** to disable and remove the timer.

## Logs

Per-job logs are written to `~/.config/dbq/logs/<job-id>.log` (or `/etc/dbq/logs/` as root). Each line records the operation, outcome, and duration:

```
2026-04-06 02:01:12  backup     ok       4m12s
2026-04-06 02:00:01  test dump  ok       3s
2026-04-05 02:01:44  backup     failed   1m02s
```

## Storage layout in S3 / R2

```
backups/
  <job-name>/
    <db-name>/
      <unix-timestamp>
```
