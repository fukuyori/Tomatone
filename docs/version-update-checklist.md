# Version Update Checklist

Use this checklist whenever the Tomatone version changes.

## Versioned files

- [ ] `cmd/tomatone/main.go`: update the `version` constant.
- [ ] `README.md`: update the displayed version.
- [ ] `docs/README.md`: update the displayed version.
- [ ] `docs/README.ja.md`: update the displayed version.
- [ ] `docs/CHANGELOG.md`: add the release version and date.

## Verification

- [ ] Search the repository for references to the previous version.
- [ ] Run `go test ./...` from PowerShell.
- [ ] Build `tomatone.exe` and confirm `tomatone.exe --version`.
- [ ] Run `git diff --check`.
- [ ] Confirm that only the intended files are changed.

`tomatone.exe` is a Git-ignored build output. Rebuild it for local verification,
but do not add it to version control.
