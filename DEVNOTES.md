TODO:

- add capability to option to paste url or search by string

DONE:

- update download progress: remove progress bar, keep percentage, add download speed + max file size + ETA + complete message
- adjust probing step: removed per-base "probing url..." lines, print selected url once after probe/challenge
- setup ci/cd & release flow (GitHub Actions + goreleaser, tag v\* -> release)
- check CF resolve first before searching
- create technical document for human with nice diagram flow (docs/ARCHITECTURE.md)
- downloaded file name should contains the resolution quality (Title - E01 - 1080p.mp4)
- ctrl+c should not showing error: interrupt (silent exit 130, prompt + download paths)
- add config download destination & remember as config (apahe config set output-dir + auto-save)
- add versioning & upgrade installed to latest capability (apahe upgrade, check-and-instruct)
- add contribution guide (CONTRIBUTING.md)
- add release workflow_dispatch (manual run) + goreleaser check/dry-run in CI to validate before tagging
