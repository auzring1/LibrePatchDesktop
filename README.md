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

`--catalog file.json` selects the catalog; `--fake` uses a sample app list so it runs off Windows.
Catalog entries must be https, carry a SHA-256, and cannot be MSI.
The catalog source (winget index or a curated list) and the graphical window are not built yet.

Windows-only code (registry scan) is built by CI but has not been run on a real PC yet.

Licensed under AGPL-3.0.
