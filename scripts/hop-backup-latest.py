#!/usr/bin/env python3
"""Print a safe summary of the newest snapshot in restic JSON input."""

import datetime
import json
import re
import sys

SAFE_ID = re.compile(r"^[A-Za-z0-9_-]+$")


def warn(message):
    print(f"PERINGATAN: {message}", file=sys.stderr)


def snapshot_id(snapshot):
    short_id = snapshot.get("short_id")
    full_id = snapshot.get("id")
    candidate = short_id if short_id is not None else full_id
    if candidate is None:
        return "unavailable"
    if not isinstance(candidate, str) or not SAFE_ID.fullmatch(candidate):
        raise ValueError("ID snapshot tidak valid")
    return candidate if short_id is not None else candidate[:8]


def main():
    try:
        snapshots = json.load(sys.stdin)
    except (json.JSONDecodeError, UnicodeDecodeError):
        warn("output snapshots bukan JSON yang valid")
        return 1
    if not isinstance(snapshots, list):
        warn("output snapshots bukan array JSON")
        return 1
    if not snapshots:
        warn("tidak ada snapshot tag hop untuk dilaporkan")
        return 1

    candidates = []
    invalid = 0
    for snapshot in snapshots:
        try:
            if not isinstance(snapshot, dict):
                raise ValueError("entri bukan objek")
            value = snapshot.get("time")
            if not isinstance(value, str):
                raise ValueError("waktu tidak tersedia")
            parsed = datetime.datetime.fromisoformat(value)
            if parsed.tzinfo is None or parsed.utcoffset() is None:
                raise ValueError("zona waktu tidak tersedia")
            parsed = parsed.astimezone(datetime.timezone.utc)
            identifier = snapshot_id(snapshot)
        except (ValueError, OverflowError):
            invalid += 1
            continue
        candidates.append((parsed, identifier))

    if invalid:
        warn(f"{invalid} entri snapshot dilewati karena waktu atau ID tidak valid")
    if not candidates:
        warn("tidak ada snapshot dengan waktu yang valid")
        return 1

    parsed, identifier = max(candidates)
    print(f"snapshot={identifier} time={parsed.isoformat()}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
