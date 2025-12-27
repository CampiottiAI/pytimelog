# lazytime

Minimal CLI time logger inspired by gtimelog. Stores entries in a single text file (default `~/.lazytime/log.txt`) using UTC timestamps and summarizes time by tags.

## Quickstart

```bash
go build -o lazytime .
./lazytime start "Write docs #project"
./lazytime status
./lazytime stop
./lazytime report             # today's totals
./lazytime report --from 2024-01-01 --to 2024-01-07
./lazytime tui                # launch terminal UI
```

Or install globally:
```bash
go install .
lazytime start "Write docs #project"
lazytime tui
```

## Commands

- `start <text> [--at TIME]` — begin an active entry; `TIME` is ISO or `HH:MM` local, default now.
- `stop [--at TIME]` — stop the active entry.
- `add --start TIME --end TIME <text>` — add a finished entry retroactively; rejects overlaps.
- `status` — show the current running entry, if any.
- `report [--from DATE] [--to DATE] [--week|--last-week]` — totals by tag for a date or range (local dates, UTC storage), with shortcuts for this or last week.
- `tui` — open a terminal UI with lazygit-like panes and shortcuts.

Tags are parsed from `#tag` words in the text. Entries without tags roll up under `(untagged)`.

## Terminal UI

Run `lazytime tui` (or `./lazytime tui` if built locally) for a split-pane terminal view (inspired by lazygit) that shows today's or this week's entries, highlights the running entry, and lets you start/stop without leaving the keyboard. Uses tcell for terminal rendering.

### Keyboard Shortcuts

- `1/2` switch between Today and Week views
- `↑/↓` scroll the active pane
- `n` starts a new entry (prompts for text)
  - Include `@HH:MM` to backdate the start time for today
  - Include two times `@HH:MM @HH:MM` to add a completed entry immediately (start/end) without leaving one running
- `x` stops the running entry
- `r` reloads the log file
- `e` or `?` shows help
- `q` or `Esc` quits

## Tagging details

- Tags are any words starting with `#` in the entry text: `Write docs #project #writing`.
- Multiple tags are allowed; time is counted toward each tag independently.
- If no tags are present, the time is grouped under `(untagged)`.
- Reports summarize by tag; tags are case-insensitive for sorting but keep their original spelling in output.

![Terminal UI screenshot](images/image.png)
