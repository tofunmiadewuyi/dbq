# MySQL backup validation — 2026-10-23

Live tests used disposable MySQL 8.4.13 and 9.7.1 containers, a separate
9.7.1 restore server, and the R2 `test` bucket. No existing application
servers or databases were modified. All cloud objects used the
`backups/mysql-e2e-20261023` prefix. Test credentials are not stored here.

## Verified cases

| Case | Result |
| --- | --- |
| MySQL 8.4, named-database streaming backup to R2 | Restored successfully |
| MySQL 9.7, GTID enabled, named-database streaming backup | Restored successfully on an isolated GTID-enabled target |
| Empty database | Uploaded and restored successfully |
| Local directory output (MySQL 9.7) | ZIP artifact restored successfully |
| SSH client-side dump/compression/upload (MySQL 9.7) | ZIP artifact restored successfully |
| SSH server-side dump/compression/upload (MySQL 9.7) | Gzip artifact restored successfully |
| Large streaming backup (MySQL 8.4) | 350,000 rows, 96,101,188 compressed bytes, two upload parts; restore matched row count, ID sum, and payload CRC32 sum |
| Special-character password | Spaces, quote, backslash and option-file metacharacters authenticated and restored successfully |
| Wrong password | Backup failed, no successful result |
| Missing PROCESS (MySQL 8.4) | Backup rejected after diagnostic fix; no cloud object or orphaned upload |
| Missing EVENT (MySQL 8.4) | Backup failed; no cloud object or orphaned upload |
| Restricted account over both SSH paths | Backup rejected; failed server-side dump file removed |
| SIGTERM during a large backup | Backup stopped; no cloud object or orphaned upload |
| Two successful backups with keep-latest 1 | Second run pruned exactly one older test backup |

The rich fixture included InnoDB and MyISAM tables, foreign keys, a generated
column, decimal and NULL values, Unicode/emoji, empty values, quotes/newlines,
and arbitrary binary bytes. Restores compared table data as raw bytes and
compared inventories of tables, views, functions, procedures, triggers and
events. The restored function was called; calling the restored procedure
caused the restored trigger to write an audit entry. A restored event was
rescheduled and observed executing on the isolated target.

## Defect found and repaired

`mysqldump` can print a metadata permission error yet exit zero. Previously,
local and SSH runners discarded successful-process stderr and certified these
backups as successful. They now recognize non-warning `mysqldump:` diagnostics
as errors. Streaming uploads abort; file-based paths reject the backup, and the
server-side SSH path removes its failed dump file. Normal GTID warnings remain
advisory. The regression test failed on all three local execution paths before
the fix and passed afterward. Live restricted-account tests verified the fix
against actual MySQL clients and R2.

## Unresolved completeness gap

**An account without SHOW_ROUTINE can silently omit procedures and functions.**
On MySQL 8.4, an account with the database SELECT, SHOW VIEW, TRIGGER, EVENT and
LOCK TABLES grants plus global PROCESS, but without SHOW_ROUTINE, produced a
successful dump with neither fixture routine. There was no stderr diagnostic.
The diagnostic fix cannot detect an object the server silently hides.

The existing documented SHOW_ROUTINE requirement is therefore material. A
future fail-closed privilege/completeness check is needed before promising a
complete backup for arbitrary accounts. Connectivity health checks currently
do not establish this. `mysqladmin ping` also allows a wrong-password request
to pass connectivity preflight; the actual dump still fails safely.

MySQL 9.7 additionally reported masking-policy metadata permission errors for
the database-scoped account. A global SELECT grant on the disposable test
account allowed the successful case. This is a test setup observation, not a
recommendation to broaden production grants without reviewing that version's
requirements.

## Boundaries

These are single-database logical backups. Table-only, schema-only and
all-databases selections are not exposed by dbq and were not claimed as tested.
Database users/global grants, external plugins and server configuration are
outside the backup. GTID restore was tested on an isolated target, not by
merging GTID history into an existing deployment. Tests did not simulate live
concurrent writes, replication recovery, custom general tablespaces, network
failure mid-part, or MySQL 5.7/MariaDB. SSH paths remain file-based.

## Automated validation

- `go test ./...` passed after the diagnostic fix.
- `go test -race ./internal/engine ./internal/reader ./internal/storage` passed.

Test cloud objects, incomplete uploads, disposable databases and containers,
and temporary helper programs were removed. Existing `custos_local` was not
changed. Source changes remain uncommitted.
