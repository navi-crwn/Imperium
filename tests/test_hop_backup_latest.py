#!/usr/bin/env python3
"""Isolated tests for the restic snapshot summary helper."""

import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]
HELPER = ROOT / "scripts" / "hop-backup-latest.py"
FIXTURES = ROOT / "tests" / "fixtures" / "backup-latest"


class SnapshotSummaryTests(unittest.TestCase):
    maxDiff = None

    def run_helper(self, source):
        with tempfile.TemporaryDirectory() as tmp:
            home = Path(tmp) / "home"
            cache = Path(tmp) / "cache"
            home.mkdir()
            cache.mkdir()
            result = subprocess.run(
                [sys.executable, "-I", str(HELPER)],
                input=source.read_bytes() if isinstance(source, Path) else source.encode(),
                capture_output=True,
                env={"HOME": str(home), "XDG_CACHE_HOME": str(cache)},
                check=False,
                timeout=5,
            )
        self.assertNotIn(str(FIXTURES), result.stdout.decode())
        self.assertNotIn(str(FIXTURES), result.stderr.decode())
        return result

    def assert_success(self, result, snapshot_id, timestamp):
        self.assertEqual(result.returncode, 0, result.stderr.decode())
        self.assertEqual(
            result.stdout.decode(),
            f"snapshot={snapshot_id} time={timestamp}\n",
        )

    def test_unsorted_multi_host_and_path_uses_chronological_latest(self):
        result = self.run_helper(FIXTURES / "mixed.json")
        self.assert_success(result, "abcdef12", "2026-10-29T23:45:00.123457+00:00")
        output = result.stdout.decode() + result.stderr.decode()
        for secret in ("private-host", "/private/path", "private-tag"):
            self.assertNotIn(secret, output)

    def test_equal_instants_have_deterministic_id_tiebreak(self):
        data = [
            {"time": "2026-10-30T01:00:00+01:00", "short_id": "11111111"},
            {"time": "2026-10-30T00:00:00Z", "short_id": "22222222"},
        ]
        self.assert_success(
            self.run_helper(json.dumps(data)), "22222222", "2026-10-30T00:00:00+00:00"
        )

    def test_missing_short_id_uses_safe_full_id_prefix(self):
        data = [{"time": "2026-10-30T00:00:00Z", "id": "0123456789abcdef"}]
        self.assert_success(
            self.run_helper(json.dumps(data)), "01234567", "2026-10-30T00:00:00+00:00"
        )

    def test_missing_id_is_reported_without_using_host_or_path(self):
        data = [{
            "time": "2026-10-30T00:00:00Z",
            "hostname": "do-not-print",
            "paths": ["do-not-print-either"],
        }]
        self.assert_success(
            self.run_helper(json.dumps(data)), "unavailable", "2026-10-30T00:00:00+00:00"
        )

    def test_invalid_entries_warn_but_valid_entry_is_selected(self):
        data = [
            {"time": "not-a-time", "short_id": "deadbeef", "paths": ["secret"]},
            {"short_id": "badc0ffe"},
            {"time": "2026-10-30T00:00:00Z", "short_id": "cafebabe"},
        ]
        result = self.run_helper(json.dumps(data))
        self.assert_success(result, "cafebabe", "2026-10-30T00:00:00+00:00")
        self.assertIn("PERINGATAN", result.stderr.decode())
        self.assertNotIn("secret", result.stderr.decode())

    def test_empty_array_is_an_explicit_error(self):
        result = self.run_helper("[]")
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(result.stdout, b"")
        self.assertIn("PERINGATAN", result.stderr.decode())

    def test_malformed_json_is_an_explicit_error(self):
        result = self.run_helper("{not-json")
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(result.stdout, b"")
        self.assertIn("PERINGATAN", result.stderr.decode())

    def test_wrong_top_level_type_is_an_explicit_error(self):
        result = self.run_helper('{"time":"2026-10-30T00:00:00Z"}')
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("PERINGATAN", result.stderr.decode())

    def test_no_valid_timestamp_is_an_explicit_error(self):
        result = self.run_helper('[{"short_id":"deadbeef"},{"time":null}]')
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(result.stdout, b"")
        self.assertIn("PERINGATAN", result.stderr.decode())

    def test_naive_timestamp_is_rejected(self):
        result = self.run_helper('[{"time":"2026-10-30T00:00:00","short_id":"deadbeef"}]')
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(result.stdout, b"")
        self.assertIn("PERINGATAN", result.stderr.decode())

    def test_unsafe_id_is_rejected_without_echoing_it(self):
        result = self.run_helper(json.dumps([{
            "time": "2026-10-30T00:00:00Z",
            "short_id": "unsafe id /private/path",
        }]))
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(result.stdout, b"")
        self.assertNotIn("/private/path", result.stderr.decode())
        self.assertIn("PERINGATAN", result.stderr.decode())

    def test_million_character_id_fails_with_bounded_generic_warning(self):
        result = self.run_helper(json.dumps([{
            "time": "2026-10-30T00:00:00Z",
            "short_id": "a" * 1_000_000,
        }]))
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(result.stdout, b"")
        self.assertLessEqual(len(result.stderr), 200)
        self.assertEqual(
            result.stderr.decode(),
            "PERINGATAN: ID snapshot terlalu panjang\n"
            "PERINGATAN: 1 entri snapshot dilewati karena waktu atau ID tidak valid\n"
            "PERINGATAN: tidak ada snapshot dengan waktu yang valid\n",
        )

    def test_65_character_valid_id_fails_without_echoing_input(self):
        result = self.run_helper(json.dumps([{
            "time": "2026-10-30T00:00:00Z",
            "short_id": "A1_-" * 16 + "A",
        }]))
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(result.stdout, b"")
        self.assertLessEqual(len(result.stderr), 200)
        self.assertEqual(
            result.stderr.decode(),
            "PERINGATAN: ID snapshot terlalu panjang\n"
            "PERINGATAN: 1 entri snapshot dilewati karena waktu atau ID tidak valid\n"
            "PERINGATAN: tidak ada snapshot dengan waktu yang valid\n",
        )

    def test_nanoseconds_determine_latest_before_id_tiebreak(self):
        data = [
            {"time": "2026-10-30T01:00:00.000000001+01:00", "short_id": "zzzzzzzz"},
            {"time": "2026-10-30T00:00:00.000000002Z", "short_id": "aaaaaaaa"},
        ]
        self.assert_success(
            self.run_helper(json.dumps(data)),
            "aaaaaaaa",
            "2026-10-30T00:00:00.000000002+00:00",
        )

    def test_nanosecond_offset_timestamp_is_normalized(self):
        data = [{"time": "2026-10-30T05:30:00.123456789+05:30", "short_id": "abcdef12"}]
        self.assert_success(
            self.run_helper(json.dumps(data)),
            "abcdef12",
            "2026-10-30T00:00:00.123456789+00:00",
        )

    def test_equal_nanosecond_instants_use_id_tiebreak(self):
        data = [
            {"time": "2026-10-30T01:00:00.123456789+01:00", "short_id": "11111111"},
            {"time": "2026-10-30T00:00:00.123456789Z", "short_id": "22222222"},
        ]
        self.assert_success(
            self.run_helper(json.dumps(data)),
            "22222222",
            "2026-10-30T00:00:00.123456789+00:00",
        )

    def test_unbounded_timestamp_fraction_fails_with_generic_warning(self):
        result = self.run_helper(json.dumps([{
            "time": "2026-10-30T00:00:00." + "1" * 10000 + "Z",
            "short_id": "deadbeef",
        }]))
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(result.stdout, b"")
        self.assertLessEqual(len(result.stderr), 300)
        self.assertNotIn("1" * 20, result.stderr.decode())
        self.assertIn("PERINGATAN", result.stderr.decode())


if __name__ == "__main__":
    unittest.main()
