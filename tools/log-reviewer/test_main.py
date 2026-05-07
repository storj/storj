"""Unit tests for log-reviewer pure functions."""

import re
import sys
import types
from pathlib import Path
from unittest.mock import MagicMock

import pytest

# Stub GCP / Vertex AI / JWT packages so we can import main without credentials.
for _mod in [
    "google", "google.cloud", "google.cloud.logging_v2", "google.cloud.storage",
    "vertexai", "vertexai.generative_models",
    "jwt", "yaml",
]:
    if _mod not in sys.modules:
        sys.modules[_mod] = MagicMock()

# yaml.safe_load must work for load_known_benign; replace the mock with real yaml.
import yaml as _real_yaml
sys.modules["yaml"] = _real_yaml

sys.path.insert(0, str(Path(__file__).parent))

from main import (
    Cluster,
    bug_group_key,
    cluster_entries,
    entry_signature,
    group_clusters,
    load_known_benign,
    match_known_benign,
    normalize_for_signature,
    sanitize_for_report,
    subsystem_for_logger,
    update_state,
    _first_meaningful_line,
    _is_stack_frame_line,
    _looks_like_json_content,
    _terminal_pkg,
)


# ---------- normalize_for_signature ----------


def test_normalize_replaces_uuid():
    s = normalize_for_signature("segment 123e4567-e89b-12d3-a456-426614174000 missing")
    assert "<UUID>" in s
    assert "123e4567" not in s


def test_normalize_replaces_timestamp():
    s = normalize_for_signature("at 2024-01-15T08:23:11.123Z bucket")
    assert "<TS>" in s
    assert "2024-01-15" not in s


def test_normalize_replaces_ip():
    s = normalize_for_signature("dial tcp 10.0.1.42:7777 refused")
    assert "<IP>" in s
    assert "10.0.1" not in s


def test_normalize_replaces_long_hex():
    s = normalize_for_signature("piece id abcdef1234567890 not found")
    assert "<HEX>" in s


def test_normalize_replaces_long_numbers():
    s = normalize_for_signature("offset 123456 length 7890123")
    assert "<N>" in s
    assert "123456" not in s


def test_normalize_preserves_short_numbers():
    s = normalize_for_signature("retry 3 of 5")
    assert "3" in s
    assert "5" in s


def test_normalize_replaces_opaque_stripe_id():
    s = normalize_for_signature("customer cus_AbC12345678 not found")
    assert "<ID>" in s
    assert "cus_AbC" not in s


# ---------- sanitize_for_report ----------


def test_sanitize_redacts_email():
    s = sanitize_for_report("user alice@example.com failed login")
    assert "<redacted:email>" in s
    assert "alice@example.com" not in s


def test_sanitize_redacts_bearer_token():
    s = sanitize_for_report("Authorization: Bearer eyJhbGciOiJSUzI1NiJ9.payload.sig")
    assert "<redacted:token>" in s
    assert "eyJhbGci" not in s


def test_sanitize_redacts_signed_url_params():
    s = sanitize_for_report("GET /object?foo=bar&signature=ABCDEF123&baz=1")
    assert "<redacted>" in s
    assert "ABCDEF123" not in s


def test_sanitize_preserves_normal_text():
    s = sanitize_for_report("context canceled: dial tcp refused")
    assert s == "context canceled: dial tcp refused"


# ---------- entry_signature ----------


def test_entry_signature_stable_across_different_uuids():
    e1 = {"payload": {"L": "ERROR", "N": "storj.io/storj/satellite/metainfo",
                      "M": "object not found", "error": "uuid: 123e4567-e89b-12d3-a456-000000000001"}}
    e2 = {"payload": {"L": "ERROR", "N": "storj.io/storj/satellite/metainfo",
                      "M": "object not found", "error": "uuid: 999aaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"}}
    sig1, _ = entry_signature(e1)
    sig2, _ = entry_signature(e2)
    assert sig1 == sig2


def test_entry_signature_differs_for_different_messages():
    e1 = {"payload": {"L": "ERROR", "M": "connection refused"}}
    e2 = {"payload": {"L": "ERROR", "M": "object not found"}}
    sig1, _ = entry_signature(e1)
    sig2, _ = entry_signature(e2)
    assert sig1 != sig2


def test_entry_signature_meta_fields():
    e = {"payload": {"L": "WARN", "N": "storj.io/storj/satellite/overlay", "M": "node offline"},
         "resource_labels": {"container_name": "satellite"}}
    sig, meta = entry_signature(e)
    assert meta["level"] == "WARN"
    assert meta["logger"] == "storj.io/storj/satellite/overlay"
    assert meta["container"] == "satellite"
    assert len(sig) == 16


# ---------- cluster_entries ----------


def _now_ts() -> str:
    import datetime as dt
    return dt.datetime.now(dt.timezone.utc).isoformat()


def _make_entry(msg: str, level: str = "ERROR", ts: str | None = None) -> dict:
    return {
        "timestamp": ts or _now_ts(),
        "payload": {"L": level, "M": msg},
        "resource_labels": {"container_name": "satellite"},
    }


def test_cluster_groups_same_message():
    entries = [_make_entry("connection refused") for _ in range(5)]
    clusters = cluster_entries(entries, max_samples=20)
    assert len(clusters) == 1
    c = next(iter(clusters.values()))
    assert c.count == 5


def test_cluster_separates_different_messages():
    entries = [_make_entry("connection refused"), _make_entry("object not found")]
    clusters = cluster_entries(entries, max_samples=20)
    assert len(clusters) == 2


def test_cluster_normalizes_uuids_to_same_group():
    entries = [
        _make_entry(f"object {i:08x}-0000-0000-0000-000000000000 not found")
        for i in range(10)
    ]
    clusters = cluster_entries(entries, max_samples=20)
    assert len(clusters) == 1


def test_cluster_respects_max_samples():
    entries = [_make_entry("error", ts=f"2024-01-01T00:00:{i:02d}Z") for i in range(30)]
    clusters = cluster_entries(entries, max_samples=5)
    c = next(iter(clusters.values()))
    assert c.count == 30
    assert len(c.samples) == 5


def test_cluster_tracks_first_last_seen():
    import datetime as dt
    base = dt.datetime.now(dt.timezone.utc).replace(microsecond=0)
    t1 = (base).isoformat()
    t2 = (base + dt.timedelta(seconds=4)).isoformat()
    t3 = (base + dt.timedelta(seconds=2)).isoformat()
    entries = [
        _make_entry("error", ts=t1),
        _make_entry("error", ts=t2),
        _make_entry("error", ts=t3),
    ]
    clusters = cluster_entries(entries, max_samples=20)
    c = next(iter(clusters.values()))
    assert c.first_seen_in_run == t1
    assert c.last_seen_in_run == t2


# ---------- subsystem_for_logger ----------


@pytest.mark.parametrize("logger,expected", [
    ("storj.io/storj/satellite/metainfo", "metainfo"),
    ("storj.io/storj/satellite/overlay", "overlay"),
    ("storj.io/storj/satellite/repair/checker", "repair"),
    ("storj.io/storj/satellite/accounting/tally", "accounting"),
    ("storj.io/storj/satellite/gc/sender", "gc"),
    ("storj.io/storj/satellite/metabase/rangedloop", "rangedloop"),
    ("storj.io/storj/satellite", "satellite-other"),
    ("storj.io/storj/private/web", "web"),
])
def test_subsystem_from_logger(logger, expected):
    assert subsystem_for_logger(logger) == expected


def test_subsystem_falls_back_to_message_stack_frame():
    result = subsystem_for_logger(
        "<unknown-logger>",
        message="goroutine panic\nstorj.io/storj/satellite/audit/worker.go:123",
    )
    assert result == "audit"


def test_subsystem_unknown_for_unrecognized_logger():
    # Falls back to last segment of the logger name.
    result = subsystem_for_logger("some.other.package")
    assert result == "package"


# ---------- match_known_benign ----------


def test_match_known_benign_hits():
    patterns = [(re.compile(re.escape("context canceled"), re.IGNORECASE), "client disconnected")]
    reason = match_known_benign("dial: context canceled", patterns)
    assert reason == "client disconnected"


def test_match_known_benign_case_insensitive():
    patterns = [(re.compile(re.escape("Context Canceled"), re.IGNORECASE), "ok")]
    assert match_known_benign("context canceled", patterns) is not None


def test_match_known_benign_no_match():
    patterns = [(re.compile(re.escape("context canceled"), re.IGNORECASE), "ok")]
    assert match_known_benign("connection refused", patterns) is None


# ---------- update_state ----------


def test_update_state_is_pure():
    original = {"clusters": {}, "last_run": "old"}
    clusters = cluster_entries([_make_entry("error")], max_samples=5)
    run_ts = _now_ts()
    new_state = update_state(original, clusters, {}, run_ts)
    assert original["last_run"] == "old"
    assert new_state["last_run"] == run_ts


def test_update_state_adds_new_cluster():
    clusters = cluster_entries([_make_entry("error")], max_samples=5)
    sig = next(iter(clusters))
    state = update_state({"clusters": {}}, clusters, {}, _now_ts())
    assert sig in state["clusters"]
    assert state["clusters"][sig]["total_count"] == 1


def test_update_state_accumulates_count():
    clusters = cluster_entries([_make_entry("error")], max_samples=5)
    sig = next(iter(clusters))
    state1 = update_state({"clusters": {}}, clusters, {}, _now_ts())
    state2 = update_state(state1, clusters, {}, _now_ts())
    assert state2["clusters"][sig]["total_count"] == 2


def test_update_state_persists_analysis():
    clusters = cluster_entries([_make_entry("error")], max_samples=5)
    sig = next(iter(clusters))
    analyses = {sig: {"summary": "test error", "urgency": "high",
                      "hypothesis": "h", "next_steps": []}}
    state = update_state({"clusters": {}}, clusters, analyses, _now_ts())
    assert state["clusters"][sig]["analysis"]["summary"] == "test error"
    assert state["clusters"][sig]["analysis"]["urgency"] == "high"


def test_update_state_evicts_old_entries():
    import datetime as dt
    old_ts = (
        dt.datetime.now(dt.timezone.utc) - dt.timedelta(days=100)
    ).isoformat()
    old_state = {"clusters": {"deadbeef": {"last_seen_ever": old_ts, "total_count": 1}}}
    clusters = cluster_entries([_make_entry("new error")], max_samples=5)
    state = update_state(old_state, clusters, {}, dt.datetime.now(dt.timezone.utc).isoformat())
    assert "deadbeef" not in state["clusters"]


# ---------- bug grouping ----------


def _cluster(template, count=1, sig="x"*16, logger="<unknown-logger>", level="ERROR"):
    return Cluster(
        signature=sig, level=level, logger=logger, container="satellite",
        message_template=template, error_template="", count=count,
    )


def test_is_stack_frame_line_detects_indent_and_offsets():
    assert _is_stack_frame_line("\tat /go/foo.go:42")
    assert _is_stack_frame_line("storj.io/storj/satellite/audit.foo")
    assert _is_stack_frame_line("/work/foo.go:99")
    assert _is_stack_frame_line("runtime.goexit+0x1d")
    assert not _is_stack_frame_line("Could not get freeze status")
    assert not _is_stack_frame_line("error retrieving payments")


def test_first_meaningful_line_skips_frames():
    msg = "\n".join([
        "Could not get freeze status",
        "storj.io/storj/satellite/accountfreeze.(*Chore).attempt",
        "\t/work/satellite/accountfreeze/billingfreezechore.go:131",
    ])
    assert _first_meaningful_line(msg) == "Could not get freeze status"


def test_first_meaningful_line_returns_none_for_pure_stack():
    msg = "storj.io/storj/satellite/metabase/changestream.ReadPartition:180"
    assert _first_meaningful_line(msg) is None


def test_first_meaningful_line_empty():
    assert _first_meaningful_line("") is None
    assert _first_meaningful_line(None) is None


def test_terminal_pkg_extraction():
    msg = "Could not get freeze status\nstorj.io/storj/satellite/accountfreeze.(*Chore).foo"
    assert _terminal_pkg(msg) == "accountfreeze"
    msg2 = "storj.io/storj/satellite/metabase/changestream.processPartition:195"
    assert _terminal_pkg(msg2) == "changestream"
    assert _terminal_pkg("no storj frame here") == ""


def test_bug_group_freeze_pair_merges():
    # Same prose first line + same subsystem → same group key.
    msg_a = "\n".join([
        "Could not get freeze status",
        "storj.io/storj/satellite/accountfreeze.(*Chore).attemptBillingFreezeWarn.func3",
        "\t/work/satellite/accountfreeze/billingfreezechore.go:131",
        "storj.io/common/sync2.(*Cycle).Run",
        "\t/go/pkg/mod/storj.io/common@v0.0.0/sync2/cycle.go:163",
    ])
    msg_b = msg_a.replace("cycle.go:163", "cycle.go:102")
    _, h_a = bug_group_key(_cluster(msg_a))
    _, h_b = bug_group_key(_cluster(msg_b))
    assert h_a == h_b


def test_bug_group_rollup_pair_stays_separate():
    msg_a = (
        "archiving node bandwidth rollups\n"
        "storj.io/storj/satellite/accounting/rolluparchive.(*Chore).ArchiveRollups\n"
        "\t/work/.../rolluparchive.go:81"
    )
    msg_b = (
        "error archiving SN and bucket bandwidth rollups\n"
        "storj.io/storj/satellite/accounting/rolluparchive.(*Chore).Run.func1\n"
        "\t/work/.../rolluparchive.go:64"
    )
    _, h_a = bug_group_key(_cluster(msg_a))
    _, h_b = bug_group_key(_cluster(msg_b))
    assert h_a != h_b


def test_bug_group_changestream_quad_collapses_to_two():
    msgs = [
        "storj.io/storj/satellite/metabase/changestream.ReadPartition:180",
        "storj.io/storj/satellite/metabase.(*SpannerAdapter).ReadChangeStreamPartition:15",
        "storj.io/storj/satellite/metabase/changestream.processPartition:195",
        "storj.io/storj/satellite/metabase/changestream.processLoop.func1.2:179",
    ]
    digests = {bug_group_key(_cluster(m))[1] for m in msgs}
    # 3 changestream-package frames merge into one digest, the SpannerAdapter
    # case lands in a separate bucket because its terminal package differs.
    assert len(digests) == 2


def test_bug_group_audit_four_distinct_keys():
    proses = [
        "error(s) during audit",
        "failed to update reputation information with audit results",
        "process",
        "can not determine if node is contained",
    ]
    msgs = [
        f"{p}\nstorj.io/storj/satellite/audit.(*Worker).func"
        for p in proses
    ]
    digests = {bug_group_key(_cluster(m))[1] for m in msgs}
    assert len(digests) == 4


def test_bug_group_subsystem_prefix_prevents_collision():
    # Two different packages logging an identical short prose must NOT merge.
    msg_a = "operation timed out\nstorj.io/storj/satellite/audit.foo"
    msg_b = "operation timed out\nstorj.io/storj/satellite/payments.bar"
    _, h_a = bug_group_key(_cluster(msg_a))
    _, h_b = bug_group_key(_cluster(msg_b))
    assert h_a != h_b


def test_group_clusters_orders_each_group_by_count_desc():
    msg = "Could not get freeze status\nstorj.io/storj/satellite/accountfreeze.(*Chore).foo"
    groups = group_clusters([
        _cluster(msg, count=2, sig="a"*16),
        _cluster(msg, count=10, sig="b"*16),
        _cluster(msg, count=5, sig="c"*16),
    ])
    assert len(groups) == 1
    only = next(iter(groups.values()))
    assert [c.count for c in only] == [10, 5, 2]


def test_group_clusters_accepts_dict():
    msg = "error\nstorj.io/storj/satellite/audit.foo"
    clusters = {"abc": _cluster(msg, count=3, sig="abc")}
    groups = group_clusters(clusters)
    assert sum(len(g) for g in groups.values()) == 1


# ---------- JSON-content detection (Stripe invoice dump dedup) ----------


def test_looks_like_json_content_detects_braces_and_brackets():
    assert _looks_like_json_content("{\"a\": 1}")
    assert _looks_like_json_content("    [1, 2, 3]")
    assert _looks_like_json_content("    {\"foo\": \"bar\"}")


def test_looks_like_json_content_detects_escaped_quotes():
    # Stripe invoice dumps have many \" sequences from JSON-in-string escaping.
    assert _looks_like_json_content('     \\"subscription\\": null,')
    assert _looks_like_json_content('s\\": {\\n    \\"invoice_item\\":')


def test_looks_like_json_content_detects_bare_property():
    assert _looks_like_json_content('"object": "invoice",')


def test_looks_like_json_content_rejects_prose():
    assert not _looks_like_json_content("Could not get freeze status")
    assert not _looks_like_json_content("error retrieving payments")
    assert not _looks_like_json_content("ranged loop failure")
    assert not _looks_like_json_content("")


def test_first_meaningful_line_skips_json_fragments():
    # Two truncated Stripe-invoice fragments — different chunks of the same
    # underlying JSON dump. Both should yield no prose so they share a key.
    msg_a = '     \\"subscription\\": null,\n              \\"license\\": false'
    msg_b = 's\\": {\\n        \\"invoice_item\\": \\"ii_xxx\\",\\n   \\"x\\": 1'
    assert _first_meaningful_line(msg_a) is None
    assert _first_meaningful_line(msg_b) is None


def test_bug_group_stripe_invoice_dump_pair_merges():
    # Same underlying bug — Stripe invoice JSON logged at ERROR. Different
    # truncation points produced different cluster signatures historically;
    # bug group key must collapse them now.
    msg_a = '     \\"subscription\\": null,\n              \\"license_fee\\": null'
    msg_b = 's\\": {\\n        \\"invoice_item\\": \\"ii_xxx\\",\\n   \\"x\\": 1'
    _, h_a = bug_group_key(_cluster(msg_a))
    _, h_b = bug_group_key(_cluster(msg_b))
    assert h_a == h_b


# ---------- run_count tracking ----------


def test_update_state_initializes_run_count_to_one():
    clusters = cluster_entries([_make_entry("error A")], max_samples=5)
    sig = next(iter(clusters))
    state = update_state({"clusters": {}}, clusters, {}, _now_ts())
    assert state["clusters"][sig]["run_count"] == 1


def test_update_state_increments_run_count_on_repeat():
    clusters = cluster_entries([_make_entry("error A")], max_samples=5)
    sig = next(iter(clusters))
    state = update_state({"clusters": {}}, clusters, {}, _now_ts())
    state = update_state(state, clusters, {}, _now_ts())
    state = update_state(state, clusters, {}, _now_ts())
    assert state["clusters"][sig]["run_count"] == 3


def test_update_state_increments_only_once_per_run():
    # Many entries → one cluster (cluster_entries dedups). update_state is
    # called once per run, so run_count should advance by exactly 1.
    entries = [_make_entry("error A") for _ in range(50)]
    clusters = cluster_entries(entries, max_samples=5)
    assert len(clusters) == 1
    sig = next(iter(clusters))
    assert clusters[sig].count == 50
    state = update_state({"clusters": {}}, clusters, {}, _now_ts())
    state = update_state(state, clusters, {}, _now_ts())
    assert state["clusters"][sig]["run_count"] == 2
