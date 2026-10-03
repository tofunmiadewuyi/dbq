# Credential-file cleanup follow-up

Database clients currently receive temporary plaintext credential files with
`0600` permissions. Normal execution removes them, but local deletion failures
are ignored, and crashes or interrupted SSH connections can leave files behind.
Remote paths already report credential cleanup failures.

## First change: report cleanup failures

- Return local credential-file deletion errors from dump and connection tests.
- Preserve the original tool error when both execution and cleanup fail.
- Apply the existing known-secret redaction before errors reach logs or Custos.
- Verify successful cleanup and combined failures without changing backup data
  or the `dbq agent` contract.

## Later, separate changes

- Forward cancellation to database commands and SSH sessions; stop commands
  before deleting credentials.
- Prefer verified memory-backed storage on Linux where available, with private
  per-operation directories (`0700`) and files (`0600`).
- Recover abandoned credential files on later runs or remote reconnects. Use an
  ownership/locking mechanism to protect active backups; age alone is insufficient.

Hard kills, host crashes and disconnected remote hosts cannot guarantee immediate
cleanup. Memory-backed storage and later recovery reduce that remaining exposure.
These follow-ups do not move scheduling or secret ownership out of Custos.
