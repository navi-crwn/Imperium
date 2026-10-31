#!/usr/bin/env python3
"""Print a safe summary of the newest snapshot in restic JSON input.

Requires Python 3.10 or newer (standard library only).
"""

import datetime
import json
import re
import sys

if sys.version_info < (3, 10):
    print("PERINGATAN: helper ringkasan membutuhkan Python 3.10+", file=sys.stderr)
    sys.exit(1)

SAFE_ID = re.compile(r"[A-Za-z0-9_-]+\Z")
RFC3339 = re.compile(
    r"(\d{4})-(\d{2})-(\d{2})T"
    r"(\d{2}):(\d{2}):(\d{2})(?:\.(\d{1,9}))?"
    r"(Z|[+-]\d{2}:\d{2})\Z"
)


def warn(message):
    print(f"PERINGATAN: {message}", file=sys.stderr)


def snapshot_id(snapshot):
    short_id = snapshot.get("short_id")
    full_id = snapshot.get("id")
    candidate = short_id if short_id is not None else full_id
    if candidate is None:
        return "unavailable"
    if not isinstance(candidate, str):
        raise ValueError("ID snapshot tidak valid")
    if len(candidate) > 64:
        raise ValueError("ID snapshot terlalu panjang")
    if not SAFE_ID.fullmatch(candidate):
        raise ValueError("ID snapshot tidak valid")
    return candidate if short_id is not None else candidate[:8]


def parse_rfc3339(value):
    """Return (epoch seconds, nanoseconds, normalized timestamp)."""
    match = RFC3339.fullmatch(value)
    if match is None:
        raise ValueError("format waktu bukan RFC3339")
    year, month, day, hour, minute, second = map(int, match.groups()[:6])
    if second > 59:
        raise ValueError("detik kabisat tidak didukung")
    fraction = match.group(7) or ""
    nanoseconds = int(fraction.ljust(9, "0")) if fraction else 0
    zone = match.group(8)
    offset = datetime.timedelta(0)
    if zone != "Z":
        offset_hour, offset_minute = map(int, zone[1:].split(":"))
        if offset_hour > 23 or offset_minute > 59:
            raise ValueError("offset zona waktu tidak valid")
        offset = datetime.timedelta(hours=offset_hour, minutes=offset_minute)
        if zone[0] == "-":
            offset = -offset
    local = datetime.datetime(year, month, day, hour, minute, second)
    utc = local - offset
    elapsed = utc - datetime.datetime(1970, 1, 1)
    seconds = elapsed.days * 86400 + elapsed.seconds
    normalized = utc.strftime("%Y-%m-%dT%H:%M:%S")
    if nanoseconds:
        normalized += "." + f"{nanoseconds:09d}".rstrip("0")
    return seconds, nanoseconds, normalized + "+00:00"


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
            if not isinstance(value, str) or len(value) > 64:
                raise ValueError("waktu tidak tersedia atau terlalu panjang")
            parsed = parse_rfc3339(value)
            identifier = snapshot_id(snapshot)
        except (ValueError, OverflowError) as error:
            if str(error) == "ID snapshot terlalu panjang":
                warn(str(error))
            invalid += 1
            continue
        candidates.append((parsed[0], parsed[1], identifier, parsed[2]))

    if invalid:
        warn(f"{invalid} entri snapshot dilewati karena waktu atau ID tidak valid")
    if not candidates:
        warn("tidak ada snapshot dengan waktu yang valid")
        return 1

    _, _, identifier, normalized = max(candidates)
    print(f"snapshot={identifier} time={normalized}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
