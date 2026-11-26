# LibrePatchDesktop

A free, open source application updater for Windows home PCs, in the spirit of Patch My PC Home Updater.
It finds the applications installed on your PC, shows which have updates, and updates them.

**Scope:** applications only. No Windows Update features (use Settings > Windows Update). LibrePatchDesktop itself ships as a plain portable exe, no installer; the apps it updates may use exe, MSI or MSIX/Appx installers.
Downloads (from GitHub Releases, built when a `v*` tag is pushed): `LibrePatchDesktop-Setup.exe`, a normal
installer (Start menu entry, optional desktop shortcut, uninstaller, Add/Remove Programs entry), and
`LibrePatchDesktop-Portable.exe` for running without installing. `SHA256SUMS.txt` lists their checksums.
Releases are not code-signed yet, so Windows SmartScreen may warn.

## Status

Early development. Working today (command line):

    librepatchdesktop list      # installed applications (Windows registry)
    librepatchdesktop check     # which have updates, per the catalog
    librepatchdesktop update    # download, verify SHA-256, install silently

Updates come from the **winget community index** (the same app database the winget client uses). The index
(`source2.msix`) is cached for a day; the installer URL, SHA-256 and silent switches are read from the package's
winget manifest just before installing. Supported: `.exe` installers (inno, nullsoft, burn, or an exe with
documented silent switches), MSI (via msiexec) and MSIX/Appx packages; zip and portable entries are skipped. Nothing
runs unless the SHA-256 matches. (This only concerns the apps being updated; LibrePatchDesktop itself ships as a plain exe.)

Catalog sources are pluggable (`internal/catalog.Source`): `--catalog file.json` adds a local list that is consulted
before winget, `--no-winget` turns winget off, `--fake` uses a sample app list off Windows. A curated catalog and
automatic discovery of vendor downloads can be added later as further sources.

The window is a native Windows window (Microsoft Edge WebView2, present on Windows 11 and most Windows 10 PCs); if WebView2 is missing it opens in the browser instead. The app asks for administrator rights at start (UAC) because most installers need them. The index code is adapted from LibrePatch (same owner, AGPL-3.0).

Not yet run for real: the registry scan on Windows, and the download of the real winget index (the build sandbox
cannot reach that host; the index reader is tested against a small sample file). Please try `check` on a PC first.

Licensed under AGPL-3.0.
