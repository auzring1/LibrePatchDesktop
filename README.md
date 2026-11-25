# LibrePatchDesktop

A free, open source application updater for Windows home PCs, in the spirit of Patch My PC Home Updater.
It finds the applications installed on your PC, shows which have updates, and updates them.

**Scope:** applications only. No Windows Update features (use Settings > Windows Update), no MSI or enterprise packaging.
Shipped as a single portable `.exe`, no installer.

## Status

Early development. Working today (command line):

    librepatchdesktop list      # installed applications (Windows registry)
    librepatchdesktop check     # which have updates, per the catalog
    librepatchdesktop update    # download, verify SHA-256, install silently

Updates come from the **winget community index** (the same app database the winget client uses). The index
(`source2.msix`) is cached for a day; the installer URL, SHA-256 and silent switches are read from the package's
winget manifest just before installing. Only plain `.exe` installers are used (inno, nullsoft, burn, or an exe with
documented silent switches); MSI/MSIX/zip/portable entries are skipped, and nothing runs unless the SHA-256 matches.

Catalog sources are pluggable (`internal/catalog.Source`): `--catalog file.json` adds a local list that is consulted
before winget, `--no-winget` turns winget off, `--fake` uses a sample app list off Windows. A curated catalog and
automatic discovery of vendor downloads can be added later as further sources.

The graphical window is not built yet. The index code is adapted from LibrePatch (same owner, AGPL-3.0).

Not yet run for real: the registry scan on Windows, and the download of the real winget index (the build sandbox
cannot reach that host; the index reader is tested against a small sample file). Please try `check` on a PC first.

Licensed under AGPL-3.0.
