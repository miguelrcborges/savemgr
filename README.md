# savemgr

A small utility to aumatically manage game saves.


# Usage

1. Create a .savemgr file in the work directory of the launcher process.
2. In that text file, define the path were the files for that application are stored in `SAVE_LOCATION`.
    - **OPTIONAL**: Define `CATBOX_USER_HASH` and `CATBOX_ALBUM` to enable cloud storage.
3. Use `savemgr` to start the process.

It integrates easily with any kind of launcher, as long as it supports launch options.

The saves path for most games are acessible [here](https://www.pcgamingwiki.com/wiki/Home).
