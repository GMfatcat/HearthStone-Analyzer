# Node / npm Path Notes

## Current Confirmed Install Paths

- `node.exe`: `C:\Program Files\nodejs\node.exe`
- `npm.cmd`: `C:\Program Files\nodejs\npm.cmd`

## Confirmed Versions

- Node.js: `v24.14.0`
- npm: `11.9.0`

## Important Environment Note

This machine has Node.js and npm installed, but some Codex / PowerShell sessions may start without
`C:\Program Files\nodejs` on `PATH`.

When that happens:

- direct `node` / `npm` commands may fail with `CommandNotFoundException`
- `npm run build` may also fail because child processes cannot resolve `node`

## Working Command Pattern For This Repo

Use either the absolute paths:

```powershell
& 'C:\Program Files\nodejs\node.exe' --version
& 'C:\Program Files\nodejs\npm.cmd' --version
```

Or prepend Node.js to `PATH` for the current shell:

```powershell
$env:PATH='C:\Program Files\nodejs;' + $env:PATH
npm run build
```

Most reliable sequence used in recent sessions:

```powershell
$env:PATH='C:\Program Files\nodejs;' + $env:PATH
& 'C:\Program Files\nodejs\npm.cmd' test
& 'C:\Program Files\nodejs\npm.cmd' run build
```

If Go tests fail with `web\embed.go: pattern dist/*: no matching files found`, rebuild frontend first:

```powershell
$env:PATH='C:\Program Files\nodejs;' + $env:PATH
& 'C:\Program Files\nodejs\npm.cmd' run build
go test ./...
```

## Why This Was Recorded

Earlier milestones already installed Node.js on this machine, but this project can still appear to
"not have node/npm" when the current shell session does not inherit the correct `PATH`.

This note should be treated as required session bootstrap context for future Codex runs on this repo.
