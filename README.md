# dbq

A CLI tool for scheduling and running PostgreSQL and MySQL backups, with support for local directory storage or cloud storage (AWS S3 / Cloudflare R2).

## Features

- Interactive terminal UI for creating and managing backup jobs
- **PostgreSQL** and **MySQL** support
- Backup to a local directory or upload directly to S3 / R2
- SSH support — dump a remote database over an SSH connection
- Server-side mode — dump and upload entirely on the server, bypassing your home internet
- Schedule backups with systemd timers (Linux) or launchd (macOS)
- Data retention — automatically prune old backups, keeping the N most recent per job
- Per-job log files with timing for every run and test


## Platform support

macOS and Linux only. Windows is not currently supported.

## Requirements

- `pg_dump` / `mysqldump` installed wherever the database lives (locally or on the SSH host)
- For scheduled backups: systemd (Linux) or launchd (macOS)
- For server-side uploads: `curl` on the remote host (installed by default on most Linux servers)

## Installation
```bash
curl -fsSL https://dbq.tofunmiadewuyi.com/install.sh | bash
```
Pin a version with `DBQ_VERSION=v1.2.3`, or point at another artifact host with `DBQ_RELEASE_BASE`. `dbq upgrade` reads from the same release base.

or if you prefer to compile yourself:

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

```
dbq start                          Open the interactive job manager
dbq run <job-id>                   Run a backup job by ID
dbq logs <job-id> [--lines <N>]   Print log history for a job
dbq config <job-id>                Print the config file for a job
dbq prune <job-id>                 Delete old backups now, per the job's retention setting
dbq delete <job-id>                Delete a job by ID
dbq upgrade                        Upgrade dbq to the latest release
dbq version / -v                   Print the current version
dbq help / -h                      Show usage
```

### Interactive mode

```bash
dbq start
```

Launches the interactive menu. From here you can create jobs, run them manually, view logs, test connections, edit, schedule/unschedule, and delete them.

> After creating a job, go to **Manage Jobs → [job name] → Schedule** to activate the backup timer. Jobs are not scheduled automatically.

### Run a job directly (used by systemd / launchd)

```bash
dbq run <job-id>
```

Runs the backup for the given job ID non-interactively. This is what the generated systemd service or launchd plist calls.

## Job configuration

Jobs are stored as TOML files in `~/.config/dbq/jobs/` (or `/etc/dbq/jobs/` when running as root). You can create and edit them through the interactive UI.
Credentials are stored separately by the selected secrets store and never
written to job TOML files. An unavailable store fails without falling back to
plaintext storage.

### Secrets providers

Standalone dbq defaults to the OS keyring. On headless Linux, select encrypted
systemd credentials in `~/.config/dbq/config.toml` (or `/etc/dbq/config.toml`
when running as root):

```toml
[secrets]
provider = "systemd"
```

Use `provider = "keyring"` to explicitly select the OS keyring. Unknown provider
names are errors. `DBQ_SECRETS_PROVIDER` overrides this setting; generated
systemd services pin their provider so they do not depend on a login environment
or subsequent changes to the interactive default.

The systemd store requires `systemd-creds` with encryption support (systemd 250+
for root, 256+ for user-scoped credentials). Setup checks a real encryption and
decryption round trip; actual support also depends on host key access, TPM
availability, and local authorization policy. Unsupported setups fail with no
fallback. Non-root credentials use `--user` and the user service manager.

Encrypted bundles are stored in the dbq `credentials/` directory with mode
`0600`, inside a `0700` directory. Saves verify decryption before atomically
replacing the previous bundle. Encryption uses systemd's `auto` mode, which
requires a suitable host key, TPM, or both; dbq never selects its unprotected
`null` or `auto-initrd` modes. Host-key encryption does not protect against an
attacker who obtains both the encrypted bundle and the host key. TPM-bound
bundles may require re-provisioning when moving to another machine.

Manual runs and connection tests decrypt into memory. Scheduled services use
`LoadCredentialEncrypted=` and read the bundle from `$CREDENTIALS_DIRECTORY`.
A missing or malformed injected credential is an error. `dbq run <job-id>` loads
only that job. Re-schedule existing jobs after changing the selected store;
changing the provider does not copy credentials between stores.

### Migrating legacy plaintext credentials

Select the destination provider in `config.toml`, then run as the account that
owns the jobs (root for `/etc/dbq`):

```bash
dbq secrets migrate
```

Pause configuration edits and scheduled backups during migration. The command
imports the old `.secrets` JSON file and passwords/access keys embedded in job
TOML into the selected keyring or systemd store. Credentials for jobs absent
from the jobs directory are also retained.

Migration validates all sources and checks all destination bundles before any
credential write. Conflicting nonempty values stop the operation without
choosing a winner; reconcile the source and destination values, then retry.
Matching values are accepted, and destination-only fields are preserved.
Unknown legacy keys, duplicate JSON accounts, invalid credential types, and
symlinked source files are rejected.

Every complete destination bundle is read back and verified before plaintext
cleanup begins. On write or verification failure, plaintext sources remain
intact. If some writes succeeded, rerunning the same command resumes safely.
The command also detects source edits made during import and stops cleanup.
Run it outside a service credential context, so verification uses the durable
store rather than an injected runtime credential.

Job configs are replaced atomically with private files after verification;
non-secret values, including custom fields, are preserved, while TOML comments
and formatting are normalized. The `.secrets` file is removed after config
cleanup. Cleanup failures can leave some already-verified configs cleaned and
other sources intact; retrying completes the remaining work. Ordinary commands
request explicit migration rather than automatically consuming plaintext.

Deleting plaintext files or fields does not erase copies in snapshots, backups,
or the underlying storage. If those copies may have been exposed, rotate the
credentials after migration. Existing keyring-to-systemd transfers are not part
of this legacy plaintext migration command.

The secrets boundary separates read-only `secrets.Provider` (load a job's
credential bundle) from writable `secrets.Store` (load, save, and delete).
The execution engine receives resolved credentials in memory and does not
initialize a store.

For external actors such as Custos, `dbq agent` uses a read-only environment
provider with the existing `DBQ_DB_PASSWORD`, `DBQ_STORAGE_ACCESS_KEY`, and
`DBQ_STORAGE_SECRET_KEY` contract. The external actor owns scheduling and
credential lifecycle; dbq only executes the request. Agent mode does not read
standalone provider settings, probe the keyring, migrate local files, or persist
supplied credentials. Database tools' temporary plaintext credential files are
unchanged in this step.

Interactive database and storage credentials are entered without terminal echo.
Editing shows an instruction to press Enter to keep the current value, never
the existing credential. Secret prompts require a terminal; external runners
continue supplying credentials through `dbq agent`.

Local database commands and local shell commands strip the three agent
credential variables plus `PGPASSWORD` and `MYSQL_PWD` from their environment.
The parent process retains its credentials, so Custos's agent contract is
unchanged. Database clients receive credential-file paths rather than passwords.

Server-side SSH dumps also use SFTP credential files and quoted command
arguments. Dumps stay on the remote server, with private file permissions;
existing output files are refused, and failed commands remove partial output.
Compression, upload, and cleanup use the same quoted argument interface.
Presigned upload URLs travel through SSH stdin into curl's configuration input,
without appearing in remote command text or process arguments or creating a URL
file. Curl's default configuration is disabled for these uploads.

### Example — local PostgreSQL database, upload to S3

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

[storage]
  provider = "AWS (S3)"
  bucket = "my-backups"
  region = "eu-west-1"
```

### Example — local MySQL database, save to directory

```toml
name = "mysql-app"
id = "mysql-app"
storage_type = "directory"
destination = "/mnt/backups"
frequency = "0 3 * * *"

[database]
  name = "mydb"
  type = "mysql"
  host = "localhost"
  port = "3306"
  username = "root"
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
```

### `useserver` mode

When `useserver = true`, dbq does not stream the dump over your SSH connection. Instead:

1. `pg_dump` / `mysqldump` runs on the server and writes to `/var/tmp/dbq/` on that host
2. A presigned PUT URL is generated on your machine (storage access keys stay there)
3. The URL is sent through SSH stdin, and the server uploads the file directly to S3/R2 via `curl`
4. The temp file is deleted from the server

This is useful when your SSH connection is slow but the server has fast outbound internet to AWS/Cloudflare.

Requirements: `curl` on the server, outbound HTTPS (port 443) allowed.

## Scheduling

From the job management menu, select **Schedule** to install a timer for the job.

**Linux (systemd)** — creates:
- `~/.config/systemd/user/dbq-<id>.service`
- `~/.config/systemd/user/dbq-<id>.timer`

The timer uses `Persistent=true`, so a missed run fires as soon as the machine comes back online.

**macOS (launchd)** — creates:
- `~/Library/LaunchAgents/com.dbq.<id>.plist`

Select **Unschedule** to disable and remove the timer.

## Logs

Per-job logs are written to `~/.config/dbq/logs/<job-id>.log` (or `/etc/dbq/logs/` as root). Each line records the operation, outcome, and duration:

```
2026-04-06 02:01:12  backup     ok       4m12s
2026-04-06 02:00:01  test dump  ok       3s
2026-04-05 02:01:44  backup     failed   1m02s
```

View logs from the CLI:
```bash
dbq logs <job-id>
dbq logs <job-id> --lines 10   # last 10 entries
```

## Inspiration

Inspired by [r2-backup](https://github.com/dej10/r2-backup).

## Storage layout in S3 / R2

```
backups/
  <job-name>/
    <db-name>/
      <unix-timestamp>
```
