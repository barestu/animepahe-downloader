TODO:

- add capability to option to paste url or search by string
- update download progress: remove progress bar, keep the percentage, add download speed
- add config download destination & remember as config
- adjust probing step, simplify by remove "probing url...", after challenge success & probe again, printout the selected url
- add versioning & upgrade installed to latest capability
- add contribution guide

DONE:

- setup ci/cd & release flow (GitHub Actions + goreleaser, tag v\* -> release)
- check CF resolve first before searching
- create technical document for human with nice diagram flow (docs/ARCHITECTURE.md)
- downloaded file name should contains the resolution quality (Title - E01 - 1080p.mp4)
- ctrl+c should not showing error: interrupt (silent exit 130, prompt + download paths)
