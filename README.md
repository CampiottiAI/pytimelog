# pytimelog

Minimal CLI time logger inspired by gtimelog. Stores entries in a single text file (default `~/.pytimelog/log.txt`) using UTC timestamps and summarizes time by tags.

## Quickstart

```bash
pip install .  # or: python -m pip install -e .
pytimelog start "Write docs #project"
pytimelog status
pytimelog stop
pytimelog report             # today's totals
pytimelog report --from 2024-01-01 --to 2024-01-07
```

## Commands

- `start <text> [--at TIME]` — begin an active entry; `TIME` is ISO or `HH:MM` local, default now.
- `stop [--at TIME]` — stop the active entry.
- `add --start TIME --end TIME <text>` — add a finished entry retroactively; rejects overlaps.
- `status` — show the current running entry, if any.
- `report [--from DATE] [--to DATE]` — totals by tag for a date or range (local dates, UTC storage).

Tags are parsed from `#tag` words in the text. Entries without tags roll up under `(untagged)`.
