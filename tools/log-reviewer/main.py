"""
Satellite log reviewer.

Daily Cloud Run Job that:
  1. Pulls the last ~26h of WARN/ERROR/FATAL logs from Cloud Logging for the
     satellite components of a given GCP project.
  2. Clusters entries by a stable message signature.
  3. Diffs against prior state in GCS to split into new / ongoing / silent.
  4. Asks Gemini for a short hypothesis on each NEW cluster.
  5. Renders a markdown report and pushes it to the configured GitHub repo
     and branch via a GitHub App installation token.

Everything is driven by environment variables (see README.md) so the same
image runs unchanged in dev and prod.
"""

from __future__ import annotations

import base64
import datetime as dt
import hashlib
import json
import logging
import os
import re
import sys
import time
from dataclasses import dataclass, field
from pathlib import Path
from typing import Iterable

import jwt
import requests
import yaml
from google.cloud import logging_v2, storage
from vertexai import init as vertex_init
from vertexai.generative_models import GenerativeModel


log = logging.getLogger("log-reviewer")
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s %(levelname)s %(name)s %(message)s",
)


# ---------- config ----------


@dataclass
class Config:
    project: str
    region: str
    state_bucket: str
    model: str
    window_hours: int
    max_entries_per_cluster: int
    max_new_clusters_to_analyze: int
    dry_run: bool
    github_app_id: str
    github_installation_id: str
    github_private_key: str
    github_repo: str
    github_branch: str
    report_dir: str
    filters_path: Path
    suppress_path: Path
    context_path: Path
    issue_threshold: int
    max_issues_per_run: int
    state_reset: bool
    max_entries: int
    read_sleep_s: float

    @classmethod
    def from_env(cls) -> "Config":
        def required(name: str) -> str:
            v = os.environ.get(name)
            if not v:
                raise RuntimeError(f"missing required env var: {name}")
            return v

        here = Path(__file__).parent
        return cls(
            project=required("GCP_PROJECT"),
            region=os.environ.get("GCP_REGION", "us-central1"),
            state_bucket=required("STATE_BUCKET"),
            model=os.environ.get("GEMINI_MODEL", "gemini-2.5-flash"),
            window_hours=int(os.environ.get("WINDOW_HOURS", "26")),
            max_entries_per_cluster=int(
                os.environ.get("MAX_ENTRIES_PER_CLUSTER", "20")
            ),
            max_new_clusters_to_analyze=int(
                os.environ.get("MAX_NEW_CLUSTERS_TO_ANALYZE", "15")
            ),
            dry_run=os.environ.get("DRY_RUN", "false").lower() == "true",
            github_app_id=required("GITHUB_APP_ID"),
            github_installation_id=required("GITHUB_INSTALLATION_ID"),
            github_private_key=required("GITHUB_PRIVATE_KEY"),
            github_repo=os.environ.get("GITHUB_REPO", "storj/qa-storj"),
            github_branch=os.environ.get("GITHUB_BRANCH", "satellite-log-reports"),
            report_dir=os.environ.get("REPORT_DIR", "reports/satellite-logs"),
            filters_path=here / "filters.yaml",
            suppress_path=here / "suppress.yaml",
            context_path=here / "context.md",
            issue_threshold=int(os.environ.get("ISSUE_THRESHOLD", "1")),
            max_issues_per_run=int(os.environ.get("MAX_ISSUES_PER_RUN", "20")),
            state_reset=os.environ.get("STATE_RESET", "false").lower() == "true",
            max_entries=int(os.environ.get("MAX_ENTRIES", "50000")),
            read_sleep_s=float(os.environ.get("READ_SLEEP_S", "1.5")),
        )


# ---------- Cloud Logging ----------


SATELLITE_CONTAINER_PATTERN = (
    'resource.type="k8s_container" '
    'AND resource.labels.container_name=~"^satellite(-|$)"'
)

# zap levels we consider "problem candidates"
ZAP_PROBLEM_LEVELS = ("ERROR", "WARN", "DPANIC", "PANIC", "FATAL")


def build_filter(cfg: Config, exclude_filters: list[str]) -> str:
    """Build the Cloud Logging filter.

    GCP's auto-severity mapping is unreliable for zap logs (ERROR is often
    mapped to WARNING), so we take the union of:
      - GCP severity>=WARNING
      - jsonPayload.L in our known zap problem levels
    """
    since = (
        dt.datetime.now(dt.timezone.utc)
        - dt.timedelta(hours=cfg.window_hours)
    ).isoformat()

    zap_levels = " OR ".join(f'jsonPayload.L="{lvl}"' for lvl in ZAP_PROBLEM_LEVELS)
    severity_union = f'(severity>=WARNING OR ({zap_levels}))'

    parts = [
        SATELLITE_CONTAINER_PATTERN,
        severity_union,
        f'timestamp>="{since}"',
    ]
    for ex in exclude_filters:
        parts.append(f"NOT ({ex})")
    return " AND ".join(parts)


def fetch_logs(cfg: Config, filter_str: str) -> list[dict]:
    """Stream log entries matching the filter and return them as plain dicts.

    Paginates manually with a small inter-page sleep so we stay under the
    Cloud Logging read quota (60 requests/minute per project, shared with
    everyone else reading the same project). A hard entries cap prevents
    an unusually chatty window from running us past the job timeout.
    """
    client = logging_v2.Client(project=cfg.project)
    entries: list[dict] = []
    max_entries = cfg.max_entries
    sleep_between_pages = cfg.read_sleep_s

    iterator = client.list_entries(
        filter_=filter_str,
        order_by=logging_v2.ASCENDING,
        page_size=1000,
    )
    # The iterator yields entries one at a time. Sleeping every PAGE_SIZE
    # entries approximates a per-page sleep without needing the pager API.
    page_size = 1000
    for i, entry in enumerate(iterator):
        if i > 0 and i % page_size == 0:
            time.sleep(sleep_between_pages)
        payload = entry.payload if isinstance(entry.payload, dict) else {
            "message": str(entry.payload)
        }
        entries.append(
            {
                "insert_id": entry.insert_id,
                "timestamp": entry.timestamp.isoformat() if entry.timestamp else None,
                "severity": str(entry.severity) if entry.severity else None,
                "resource_labels": dict(entry.resource.labels)
                if entry.resource
                else {},
                "payload": payload,
            }
        )
        if len(entries) >= max_entries:
            log.warning(
                "hit MAX_ENTRIES cap (%d); stopping early — "
                "tighten filters.yaml if this happens regularly",
                max_entries,
            )
            break
    log.info("fetched %d entries", len(entries))
    return entries


# ---------- clustering ----------


_UUID_RE = re.compile(r"\b[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\b", re.I)
_HEX_RE = re.compile(r"\b[0-9a-f]{16,}\b", re.I)
_IP_RE = re.compile(r"\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}(?::\d+)?\b")
_TS_RE = re.compile(r"\b\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?Z?\b")
_NUM_RE = re.compile(r"\b\d{4,}\b")  # only long numbers — short ones are often enum codes
# Real base64 strings include `+`, `/`, `=` — but Go package paths and stack
# frame text also contain `/` and `.` and run long. Restrict to character
# classes that don't appear in package paths so we redact tokens, not code.
_BASE64ISH_RE = re.compile(r"(?<![A-Za-z0-9+/=._-])[A-Za-z0-9+=]{32,}(?![A-Za-z0-9+/=._-])")
# Stripe-style opaque IDs: req_/cus_/sub_/in_/ii_/ch_/prod_/price_/acct_ + 10+ chars
_OPAQUE_ID_RE = re.compile(r"\b(req|cus|sub|in|ii|ch|prod|price|acct|pi|pm|tok|txn|re)_[A-Za-z0-9]{8,}\b")

# PII redaction (applied before the sample goes into the report)
_EMAIL_RE = re.compile(r"\b[\w.+-]+@[\w-]+\.[A-Za-z]{2,24}\b")
_BEARER_RE = re.compile(r"(?i)(bearer\s+)[A-Za-z0-9._-]+")
_SIGNED_URL_QS_RE = re.compile(r"(?i)([?&](?:signature|token|key|sig)=)[^&\s]+")
# Stack-frame line numbers (".go:42") vary between callers of the same log
# site — different goroutine paths through sync2.Cycle.Run land at different
# lines but log the same error. Normalize to <L> so clustering merges them.
_STACK_LINE_RE = re.compile(r"(\.go):\d+")


def normalize_for_signature(s: str) -> str:
    s = _UUID_RE.sub("<UUID>", s)
    s = _TS_RE.sub("<TS>", s)
    s = _IP_RE.sub("<IP>", s)
    s = _BASE64ISH_RE.sub("<B64>", s)
    s = _OPAQUE_ID_RE.sub("<ID>", s)
    s = _HEX_RE.sub("<HEX>", s)
    s = _STACK_LINE_RE.sub(r"\1:<L>", s)
    s = _NUM_RE.sub("<N>", s)
    return s.strip()


def sanitize_for_report(s: str) -> str:
    s = _EMAIL_RE.sub("<redacted:email>", s)
    s = _BEARER_RE.sub(r"\1<redacted:token>", s)
    s = _SIGNED_URL_QS_RE.sub(r"\1<redacted>", s)
    return s


_SUBSYSTEM_PREFIXES: list[tuple[str, str]] = [
    ("storj.io/storj/satellite/metainfo", "metainfo"),
    ("storj.io/storj/satellite/metabase/rangedloop", "rangedloop"),
    ("storj.io/storj/satellite/metabase", "metabase"),
    ("storj.io/storj/satellite/overlay", "overlay"),
    ("storj.io/storj/satellite/repair", "repair"),
    ("storj.io/storj/satellite/audit", "audit"),
    ("storj.io/storj/satellite/accounting", "accounting"),
    ("storj.io/storj/satellite/payments", "payments"),
    ("storj.io/storj/satellite/console", "console"),
    ("storj.io/storj/satellite/gc", "gc"),
    ("storj.io/storj/satellite/nodeevents", "nodeevents"),
    ("storj.io/storj/satellite/gracefulexit", "gracefulexit"),
    ("storj.io/storj/satellite/analytics", "analytics"),
    ("storj.io/storj/satellite/orders", "orders"),
    ("storj.io/storj/satellite/contact", "contact"),
    ("storj.io/storj/satellite/reputation", "reputation"),
    ("storj.io/storj/satellite/admin", "admin"),
    ("storj.io/storj/satellite/emission", "emission"),
    ("storj.io/storj/satellite/compensation", "compensation"),
    ("storj.io/storj/satellite/accountfreeze", "accountfreeze"),
    ("storj.io/storj/satellite/buckets", "buckets"),
    ("storj.io/storj/satellite", "satellite-other"),
    ("storj.io/storj/private/web", "web"),
    ("storj.io/storj/private", "private"),
    ("storj.io/storj/shared", "shared"),
]


_STORJ_FRAME_RE = re.compile(r"storj\.io/storj/[A-Za-z0-9_/\-]+")
_FRAME_OFFSET_RE = re.compile(r"\+0x[0-9a-f]+$")


def _first_storj_frame(text: str) -> str | None:
    """Return the first storj.io/storj/* package path embedded in text, if any."""
    if not text:
        return None
    m = _STORJ_FRAME_RE.search(text)
    return m.group(0) if m else None


def _is_stack_frame_line(line: str) -> bool:
    """Heuristic: is this line part of a Go stack trace rather than prose?

    True when the line is indented (zap stacktrace), references a known
    storj.io package, looks like ``foo.go:42``, or ends in ``+0xNN`` (Go
    runtime frame offset).
    """
    if line.startswith("\t"):
        return True
    if _STORJ_FRAME_RE.search(line):
        return True
    if _STACK_LINE_RE.search(line):
        return True
    return bool(_FRAME_OFFSET_RE.search(line))


def _first_meaningful_line(message: str) -> str | None:
    """Return the first non-empty, non-stack-frame line of a message, if any.

    Used to extract the human-readable log statement (the prose) from a
    multi-line zap error that includes a Go stack trace. Returns None if
    every non-empty line looks like a frame — e.g. ``metabase/changestream``
    errors whose only content is a package path with line numbers.
    """
    if not message:
        return None
    for raw in message.splitlines():
        s = raw.strip()
        if not s:
            continue
        if _is_stack_frame_line(s):
            continue
        return s
    return None


def _terminal_pkg(message: str) -> str:
    """Last path segment of the first storj.io frame's package, e.g. 'changestream'."""
    frame = _first_storj_frame(message)
    if not frame:
        return ""
    last = frame.rsplit("/", 1)[-1]
    return last.split(".", 1)[0]


def subsystem_for_logger(logger: str, message: str = "") -> str:
    """Map a logger name (or fallback message stack frame) to a subsystem label.

    Most satellite call sites don't .Named() their zap logger, so the raw
    logger field is "<unknown-logger>". When that happens, scan the message
    for the first storj.io/storj package path — typically the top of the
    Go stack trace — and derive the subsystem from there.
    """
    candidates: list[str] = []
    if logger and logger != "<unknown-logger>":
        candidates.append(logger)
    frame = _first_storj_frame(message)
    if frame:
        candidates.append(frame)
    for cand in candidates:
        for prefix, name in _SUBSYSTEM_PREFIXES:
            if cand.startswith(prefix):
                return name
    if candidates:
        tail = candidates[0].rsplit("/", 1)[-1].rsplit(".", 1)[-1]
        return tail or "unknown"
    return "unknown"


def entry_signature(entry: dict) -> tuple[str, dict]:
    """Return (signature, enriched metadata) for clustering."""
    p = entry.get("payload") or {}
    level = p.get("L") or entry.get("severity") or "UNKNOWN"
    logger_name = p.get("N") or "<unknown-logger>"
    message = p.get("M") or p.get("message") or ""
    err = p.get("error") or p.get("err") or ""

    # Cluster by code site (logger + level + message body), not by wrapped error
    # contents. Variations in the wrapped err (user IDs, partition IDs, DB error
    # detail) used to split clusters that share an identical log site, producing
    # near-duplicate GitHub issues. Sample diversity inside the cluster still
    # preserves the variation for human investigation.
    raw_key = f"{logger_name}|{level}|{normalize_for_signature(str(message))}"
    sig = hashlib.sha1(raw_key.encode("utf-8"), usedforsecurity=False).hexdigest()[:16]
    meta = {
        "level": level,
        "logger": logger_name,
        "message_template": normalize_for_signature(str(message)),
        "error_template": normalize_for_signature(str(err)),
        "container": entry.get("resource_labels", {}).get("container_name", "?"),
    }
    return sig, meta


@dataclass
class Cluster:
    signature: str
    level: str
    logger: str
    container: str
    message_template: str
    error_template: str
    count: int = 0
    first_seen_in_run: str | None = None
    last_seen_in_run: str | None = None
    samples: list[dict] = field(default_factory=list)

    def to_dict(self) -> dict:
        return {
            "signature": self.signature,
            "level": self.level,
            "logger": self.logger,
            "container": self.container,
            "message_template": self.message_template,
            "error_template": self.error_template,
            "count": self.count,
            "first_seen_in_run": self.first_seen_in_run,
            "last_seen_in_run": self.last_seen_in_run,
        }


def cluster_entries(entries: list[dict], max_samples: int) -> dict[str, Cluster]:
    clusters: dict[str, Cluster] = {}
    for e in entries:
        sig, meta = entry_signature(e)
        c = clusters.get(sig)
        if c is None:
            c = Cluster(
                signature=sig,
                level=meta["level"],
                logger=meta["logger"],
                container=meta["container"],
                message_template=meta["message_template"],
                error_template=meta["error_template"],
            )
            clusters[sig] = c
        c.count += 1
        ts = e.get("timestamp")
        if ts:
            if c.first_seen_in_run is None or ts < c.first_seen_in_run:
                c.first_seen_in_run = ts
            if c.last_seen_in_run is None or ts > c.last_seen_in_run:
                c.last_seen_in_run = ts
        if len(c.samples) < max_samples:
            c.samples.append(e)
    log.info("produced %d clusters", len(clusters))
    return clusters


# ---------- bug grouping ----------
#
# Two layers of identity drive the reviewer. A *cluster* is a granular
# log-site identity (logger + level + normalized message body). A *bug
# group* is a coarser root-cause identity used for GitHub issue dedup —
# two log call sites that surface the same underlying incident (a cascade
# wrapper plus its inner failure, or four functions in metabase/changestream
# all firing during one Spanner outage) collapse into one bug group so the
# tracker shows one ticket per bug, not one ticket per log line.


def bug_group_key(cluster: "Cluster") -> tuple[str, str]:
    """Return (human key, 8-char hex digest) for issue-level dedup.

    Strategy:
      * If the cluster's message has a meaningful prose first line (the
        actual log statement, not a stack frame), key by subsystem + that
        prose. Different prose at the same site = different bug.
      * Otherwise (the message body is purely a Go stack trace, e.g.
        metabase/changestream errors), fall back to subsystem plus the
        terminal package segment of the first storj.io frame. Stops two
        unrelated stack-only errors in different packages from accidentally
        merging into a single subsystem bucket.
    """
    subsystem = subsystem_for_logger(cluster.logger, cluster.message_template)
    prose = _first_meaningful_line(cluster.message_template)
    if prose:
        key = f"{subsystem}|{normalize_for_signature(prose)[:200]}"
    else:
        key = f"{subsystem}|<frames>|{_terminal_pkg(cluster.message_template)}"
    digest = hashlib.sha1(key.encode("utf-8"), usedforsecurity=False).hexdigest()[:8]
    return key, digest


def group_clusters(clusters: "dict[str, Cluster] | list[Cluster]") -> dict[str, list["Cluster"]]:
    """Group clusters by bug_group_key digest. Each group is sorted by -count.

    Accepts either the per-signature dict produced by cluster_entries or a
    plain list — both are common at call sites.
    """
    iterable = clusters.values() if isinstance(clusters, dict) else clusters
    groups: dict[str, list["Cluster"]] = {}
    for c in iterable:
        _, digest = bug_group_key(c)
        groups.setdefault(digest, []).append(c)
    for v in groups.values():
        v.sort(key=lambda c: -c.count)
    return groups


# ---------- state in GCS ----------


STATE_OBJECT = "cluster-state.json"
STATE_RETENTION_DAYS = 90


def _state_blob(cfg: Config):
    client = storage.Client(project=cfg.project)
    bucket = client.bucket(cfg.state_bucket)
    return bucket.blob(STATE_OBJECT)


def load_state(cfg: Config) -> dict:
    blob = _state_blob(cfg)
    if not blob.exists():
        return {"clusters": {}}
    try:
        return json.loads(blob.download_as_text())
    except Exception as exc:  # corrupt state — don't wedge the pipeline
        log.warning("state load failed (%s), starting fresh", exc)
        return {"clusters": {}}


def save_state(cfg: Config, state: dict) -> None:
    blob = _state_blob(cfg)
    blob.upload_from_string(json.dumps(state, indent=2), content_type="application/json")


def update_state(
    state: dict,
    clusters: dict[str, Cluster],
    analyses: dict[str, dict],
    run_ts: str,
) -> dict:
    """Return a new state dict with clusters seen this run merged in.

    Persisting analyses lets ongoing clusters carry their hypothesis forward
    so each daily report can stand alone — readers don't need yesterday's
    file to understand what a cluster signature means.
    """
    known = dict(state.get("clusters", {}))
    for sig, c in clusters.items():
        existing = dict(known.get(sig) or {})
        if not existing:
            existing = {
                "signature": sig,
                "logger": c.logger,
                "level": c.level,
                "message_template": c.message_template,
                "first_seen_ever": c.first_seen_in_run or run_ts,
                "last_seen_ever": c.last_seen_in_run or run_ts,
                "total_count": c.count,
            }
        else:
            existing["last_seen_ever"] = c.last_seen_in_run or run_ts
            existing["total_count"] = existing.get("total_count", 0) + c.count
        a = analyses.get(sig)
        if a and a.get("summary"):
            existing["analysis"] = {
                "summary": a.get("summary", ""),
                "urgency": a.get("urgency", ""),
                "hypothesis": a.get("hypothesis", ""),
                "next_steps": a.get("next_steps", []),
                "analyzed_at": run_ts,
            }
        known[sig] = existing

    # evict clusters not seen in STATE_RETENTION_DAYS
    cutoff = (
        dt.datetime.now(dt.timezone.utc)
        - dt.timedelta(days=STATE_RETENTION_DAYS)
    ).isoformat()
    known = {
        sig: v
        for sig, v in known.items()
        if (v.get("last_seen_ever") or run_ts) >= cutoff
    }
    return {"clusters": known, "last_run": run_ts}


def cached_analysis(state: dict, sig: str) -> dict | None:
    """Return previously stored AI analysis for a cluster signature, if any."""
    entry = state.get("clusters", {}).get(sig)
    if not entry:
        return None
    a = entry.get("analysis")
    if a and a.get("summary"):
        return a
    return None


# ---------- suppression / filter configs ----------


def load_filters(path: Path) -> list[str]:
    if not path.exists():
        return []
    data = yaml.safe_load(path.read_text()) or {}
    return [item["filter"] for item in (data.get("exclude") or []) if "filter" in item]


def load_suppress(path: Path) -> set[str]:
    if not path.exists():
        return set()
    data = yaml.safe_load(path.read_text()) or {}
    return {item["signature"] for item in (data.get("suppressed") or []) if "signature" in item}


def load_known_benign(context_path: Path) -> list[tuple[re.Pattern, str]]:
    """Read known-benign substring patterns from a YAML block embedded in context.md.

    Format inside context.md:

        <!-- known_benign:
        - pattern: "context canceled"
          reason: "client disconnected; benign"
        - pattern: "Monthly bandwidth limit exceeded"
          reason: "user quota; expected"
        -->

    Patterns are case-insensitive substring matches against the cluster's
    message_template. We embed YAML in a comment so the file reads naturally
    as documentation when humans skim it, but stays trivially parseable.
    """
    if not context_path.exists():
        return []
    text = context_path.read_text()
    m = re.search(r"<!--\s*known_benign:\s*\n(.*?)\n\s*-->", text, re.DOTALL)
    if not m:
        return []
    try:
        items = yaml.safe_load(m.group(1)) or []
    except Exception as exc:
        log.warning("could not parse known_benign block: %s", exc)
        return []
    out: list[tuple[re.Pattern, str]] = []
    for item in items:
        pat = item.get("pattern")
        reason = item.get("reason") or ""
        if not pat:
            continue
        out.append((re.compile(re.escape(pat), re.IGNORECASE), reason))
    return out


def match_known_benign(
    message_template: str, patterns: list[tuple[re.Pattern, str]]
) -> str | None:
    """Return the matching reason string, or None if no pattern matches."""
    for rx, reason in patterns:
        if rx.search(message_template):
            return reason
    return None


# ---------- Gemini hypothesis ----------


HYPOTHESIS_PROMPT = """You are a senior Storj satellite SRE triaging a newly-seen log cluster.

Known-benign patterns (context canceled, quota limits, node churn, etc.) have
already been filtered out — every cluster you receive is believed to be
non-trivial and worth investigating.

Given the cluster metadata and a few sanitized sample entries, produce:
1. A one-line summary (<=100 chars).
2. An urgency classification: "critical" (data loss / revenue impact / service
   down), "high" (degraded reliability, affects users), "medium" (background
   noise that warrants a ticket), or "low" (cosmetic / very low volume).
3. A short hypothesis about the likely root cause (2-4 sentences). Reference
   the subsystem when helpful. Do NOT invent file paths or function names you
   cannot derive from the provided fields.
4. 2-3 concrete investigation steps.

Respond ONLY as JSON:
{{
  "summary": "...",
  "urgency": "critical|high|medium|low",
  "hypothesis": "...",
  "next_steps": ["...", "..."]
}}

Cluster:
- subsystem: {subsystem}
- logger: {logger}
- level: {level}
- container: {container}
- message_template: {message_template}
- error_template: {error_template}
- occurrences_this_run: {count}

Sanitized samples:
{samples}
"""


class GeminiUnavailable(RuntimeError):
    """Raised when Gemini fails enough times in a row that the run cannot
    produce a useful report. The pipeline should abort rather than silently
    write stub hypotheses for every cluster."""


_CONSECUTIVE_FAILURE_LIMIT = 3


class _Analyzer:
    """Wrap analyze_cluster() with a consecutive-failure circuit breaker.

    On the first 3 back-to-back failures we raise GeminiUnavailable so the
    job exits visibly. Any successful call resets the counter, so transient
    blips don't kill a run.
    """

    def __init__(self, cfg: Config, model: GenerativeModel, codebase_context: str):
        self.cfg = cfg
        self.model = model
        self.codebase_context = codebase_context
        self.consecutive_failures = 0

    _RESPONSE_SCHEMA = {
        "type": "object",
        "properties": {
            "summary": {"type": "string"},
            "urgency": {"type": "string", "enum": ["critical", "high", "medium", "low"]},
            "hypothesis": {"type": "string"},
            "next_steps": {"type": "array", "items": {"type": "string"}},
        },
        "required": ["summary", "urgency", "hypothesis", "next_steps"],
    }

    def analyze(self, cluster: Cluster) -> dict:
        subsystem = subsystem_for_logger(cluster.logger, cluster.message_template)
        samples_text = "\n".join(
            "- " + sanitize_for_report(json.dumps(s.get("payload", {}), ensure_ascii=False))[:1000]
            for s in cluster.samples[:5]
        )
        context_section = (
            f"\n## Codebase context\n{self.codebase_context}\n---\n"
            if self.codebase_context else ""
        )
        prompt = (context_section + HYPOTHESIS_PROMPT).format(
            subsystem=subsystem,
            logger=cluster.logger,
            level=cluster.level,
            container=cluster.container,
            message_template=cluster.message_template,
            error_template=cluster.error_template,
            count=cluster.count,
            samples=samples_text,
        )
        try:
            resp = self.model.generate_content(
                prompt,
                generation_config={
                    "temperature": 0.2,
                    "response_mime_type": "application/json",
                },
            )
            self.consecutive_failures = 0
            return json.loads(resp.text)
        except Exception as exc:
            self.consecutive_failures += 1
            log.warning(
                "gemini analysis failed for %s (%d/%d consecutive): %s",
                cluster.signature, self.consecutive_failures,
                _CONSECUTIVE_FAILURE_LIMIT, exc,
            )
            if self.consecutive_failures >= _CONSECUTIVE_FAILURE_LIMIT:
                raise GeminiUnavailable(
                    f"Gemini analysis failed {self.consecutive_failures} times in a row "
                    f"(model={self.cfg.model}); last error: {exc}"
                ) from exc
            return {
                "summary": cluster.message_template[:100],
                "urgency": "medium",
                "hypothesis": f"(automatic analysis failed: {exc})",
                "next_steps": ["review sample logs manually"],
            }


# ---------- rendering ----------


def classify_new(clusters: dict[str, Cluster], state: dict) -> set[str]:
    """Return the set of cluster signatures that haven't been seen in prior state.

    Used to badge clusters as NEW vs ongoing in the report. The lifecycle
    distinction no longer drives the report layout — it's just an annotation.
    """
    known = set(state.get("clusters", {}).keys())
    return {sig for sig in clusters if sig not in known}


_TOP_ISSUES_MIN_COUNT = 5
_TABLE_MSG_WIDTH = 70


def _table_safe(s: str, width: int = _TABLE_MSG_WIDTH) -> str:
    """Make a string safe for a markdown table cell.

    Strips newlines, collapses whitespace, escapes pipes, truncates.
    Tables in our existing report are broken because messages contain literal
    newlines — splitting the row across multiple lines.
    """
    s = " ".join(s.split())
    s = s.replace("|", "\\|")
    if len(s) > width:
        s = s[: width - 1] + "…"
    return s


def _cluster_title(cluster: Cluster, analysis: dict) -> str:
    """Pick a readable one-line title for a cluster.

    Prefer the AI summary; fall back to the first line of the message
    template; never spill a full stack trace into a heading.
    """
    summary = (analysis or {}).get("summary")
    if summary:
        return summary.strip()
    first_line = cluster.message_template.split("\n", 1)[0].strip()
    return first_line[:120] if first_line else cluster.signature


def _badge(label: str) -> str:
    return f"`[{label}]`"


def _render_cluster_block(
    cluster: Cluster,
    analysis: dict,
    subsystem: str,
    benign_reason: str,
    is_new: bool,
    state_entry: dict | None,
) -> list[str]:
    """Render a full per-cluster section: heading, metadata, hypothesis, samples."""
    lines: list[str] = []
    title = sanitize_for_report(_cluster_title(cluster, analysis))
    lines.append(f"### `{cluster.signature[:8]}` — {title}")
    lines.append("")

    urgency = (analysis or {}).get("urgency", "")
    badges = [_badge(subsystem), _badge(cluster.level), _badge("NEW" if is_new else "ongoing")]
    if urgency:
        badges.append(_badge(urgency))
    if benign_reason:
        badges.append(_badge("known-benign"))
    lines.append(" ".join(badges))
    lines.append("")

    lines.append(f"- **count this run**: {cluster.count}")
    lines.append(f"- **subsystem**: `{subsystem}`")
    if cluster.logger and cluster.logger != "<unknown-logger>":
        lines.append(f"- **logger**: `{cluster.logger}`")
    lines.append(f"- **container**: `{cluster.container}`")
    if cluster.first_seen_in_run or cluster.last_seen_in_run:
        lines.append(
            f"- **first/last seen (run)**: {cluster.first_seen_in_run} / {cluster.last_seen_in_run}"
        )
    if state_entry and state_entry.get("first_seen_ever"):
        lines.append(
            f"- **first seen ever**: {state_entry['first_seen_ever']} "
            f"(total observed: {state_entry.get('total_count', '?')})"
        )
    if benign_reason:
        lines.append(f"- **why benign**: {benign_reason}")
    lines.append("")

    if analysis.get("hypothesis"):
        lines.append("**Hypothesis.** " + sanitize_for_report(analysis["hypothesis"]))
        lines.append("")
    if analysis.get("next_steps"):
        lines.append("**Next steps:**")
        for step in analysis["next_steps"]:
            lines.append(f"- {sanitize_for_report(str(step))}")
        lines.append("")

    if cluster.samples:
        lines.append("<details><summary>Sanitized samples</summary>")
        lines.append("")
        lines.append("```json")
        for s in cluster.samples[:5]:
            sample = sanitize_for_report(
                json.dumps(s.get("payload", {}), ensure_ascii=False)
            )
            lines.append(sample[:2000])
        lines.append("```")
        lines.append("</details>")
        lines.append("")
    return lines


def render_report(
    cfg: Config,
    run_date: dt.date,
    total_entries: int,
    clusters_this_run: list[Cluster],
    analyses: dict[str, dict],
    new_signatures: set[str],
    benign: dict[str, str],
    suppressed_sigs: set[str],
    state: dict,
) -> str:
    """Render a stand-alone daily report.

    Layout: TL;DR → Top issues (count >= 5, with full AI analysis) →
    By-subsystem tables → Known-benign / suppressed (collapsed). Each
    cluster carries its hypothesis from state when no fresh analysis ran,
    so the report is readable on its own without yesterday's context.
    """
    # Pre-compute subsystem once per cluster to avoid repeated inference.
    cluster_subsystems = {
        c.signature: subsystem_for_logger(c.logger, c.message_template)
        for c in clusters_this_run
    }

    # Partition clusters into top issues, per-subsystem buckets, benign/suppressed.
    top_issues: list[Cluster] = []
    benign_or_suppressed: list[Cluster] = []
    by_subsystem: dict[str, list[Cluster]] = {}

    sorted_clusters = sorted(clusters_this_run, key=lambda c: -c.count)
    for c in sorted_clusters:
        sub = cluster_subsystems[c.signature]
        is_benign = c.signature in benign or c.signature in suppressed_sigs
        if is_benign:
            benign_or_suppressed.append(c)
            continue
        if c.count >= _TOP_ISSUES_MIN_COUNT:
            top_issues.append(c)
        by_subsystem.setdefault(sub, []).append(c)

    # Sort top issues: urgency first (critical > high > medium > low), then count.
    _URGENCY_ORDER = {"critical": 0, "high": 1, "medium": 2, "low": 3, "": 4}
    top_issues.sort(
        key=lambda c: (
            _URGENCY_ORDER.get((analyses.get(c.signature) or {}).get("urgency", ""), 4),
            -c.count,
        )
    )

    # Subsystem ranking by total error count (excluding benign).
    subsystem_totals = sorted(
        ((sub, sum(c.count for c in cs)) for sub, cs in by_subsystem.items()),
        key=lambda t: -t[1],
    )

    # Bug groups across non-benign clusters — used by both the TL;DR
    # callout and the "By bug group" table later in the report.
    actionable_for_groups = [
        c for c in clusters_this_run
        if c.signature not in benign and c.signature not in suppressed_sigs
    ]
    bug_groups = group_clusters(actionable_for_groups)
    multi_cluster_groups = {h: g for h, g in bug_groups.items() if len(g) > 1}

    lines: list[str] = []
    lines.append(f"# Satellite logs review — {run_date.isoformat()}")
    lines.append("")
    lines.append(
        f"_project: `{cfg.project}`  ·  window: {cfg.window_hours}h  ·  "
        f"entries scanned: {total_entries}_"
    )
    lines.append("")

    # TL;DR
    lines.append("## TL;DR")
    lines.append("")
    new_count = len(new_signatures & {c.signature for c in clusters_this_run})
    lines.append(
        f"- {len(clusters_this_run)} distinct clusters this window "
        f"(**{new_count} new**, {len(clusters_this_run) - new_count} previously seen)"
    )
    lines.append(
        f"- **{len(top_issues)} clusters need attention** (count ≥ {_TOP_ISSUES_MIN_COUNT}); "
        f"{len(benign_or_suppressed)} routed to known-benign"
    )
    if subsystem_totals:
        top3 = ", ".join(f"`{sub}` ({n})" for sub, n in subsystem_totals[:3])
        lines.append(f"- top subsystems by error count: {top3}")
    if bug_groups:
        merged = sum(len(g) - 1 for g in multi_cluster_groups.values())
        lines.append(
            f"- {len(bug_groups)} distinct bug groups across actionable clusters"
            + (f" ({len(multi_cluster_groups)} merge {merged} cluster"
               f"{'s' if merged != 1 else ''} into one ticket)" if multi_cluster_groups else "")
        )
    if not top_issues:
        lines.append("- ✅ no high-volume anomalies this window")
    lines.append("")

    # Top issues
    lines.append(f"## Top issues ({len(top_issues)})")
    lines.append("")
    if not top_issues:
        lines.append(f"_No clusters reached the count ≥ {_TOP_ISSUES_MIN_COUNT} threshold this window._")
        lines.append("")
    for c in top_issues:
        lines.extend(_render_cluster_block(
            cluster=c,
            analysis=analyses.get(c.signature, {}),
            subsystem=cluster_subsystems[c.signature],
            benign_reason="",
            is_new=c.signature in new_signatures,
            state_entry=state.get("clusters", {}).get(c.signature),
        ))

    # By-bug-group table — surfaces cases where multiple clusters share a
    # root cause and file as one issue (already computed above for the TL;DR).
    lines.append(f"## By bug group ({len(bug_groups)} groups, {len(multi_cluster_groups)} multi-cluster)")
    lines.append("")
    if not bug_groups:
        lines.append("_None._")
        lines.append("")
    else:
        lines.append("| group | subsystem | clusters | combined count | primary cluster |")
        lines.append("|---|---|---:|---:|---|")
        ranked = sorted(
            bug_groups.items(),
            key=lambda kv: (-len(kv[1]), -sum(c.count for c in kv[1])),
        )
        for group_hash, group in ranked:
            primary = group[0]
            sub = subsystem_for_logger(primary.logger, primary.message_template)
            combined = sum(c.count for c in group)
            primary_summary = _table_safe(
                _cluster_title(primary, analyses.get(primary.signature, {}))
            )
            lines.append(
                f"| `{group_hash}` | {sub} | {len(group)} | {combined} | "
                f"`{primary.signature[:8]}` — {primary_summary} |"
            )
        lines.append("")

    # By-subsystem tables
    lines.append("## By subsystem")
    lines.append("")
    if not by_subsystem:
        lines.append("_None._")
        lines.append("")
    for sub, total in subsystem_totals:
        cs = by_subsystem[sub]
        lines.append(f"### {sub} — {len(cs)} clusters, {total} occurrences")
        lines.append("")
        lines.append("| sig | level | count | new? | summary |")
        lines.append("|---|---|---:|:---:|---|")
        for c in sorted(cs, key=lambda x: -x.count):
            a = analyses.get(c.signature, {})
            summary = _table_safe(_cluster_title(c, a))
            new_mark = "🆕" if c.signature in new_signatures else ""
            lines.append(
                f"| `{c.signature[:8]}` | {c.level} | {c.count} | {new_mark} | {summary} |"
            )
        lines.append("")

    # Known-benign / suppressed
    lines.append(f"## Known-benign / suppressed ({len(benign_or_suppressed)})")
    lines.append("")
    if not benign_or_suppressed:
        lines.append("_None._")
        lines.append("")
    else:
        lines.append(
            "<details><summary>Clusters matching known-benign patterns "
            "or explicitly suppressed (click to expand)</summary>"
        )
        lines.append("")
        lines.append("| sig | subsystem | level | count | reason | message |")
        lines.append("|---|---|---|---:|---|---|")
        for c in sorted(benign_or_suppressed, key=lambda x: -x.count):
            sub = cluster_subsystems[c.signature]
            reason = benign.get(c.signature) or ("suppressed" if c.signature in suppressed_sigs else "")
            msg = _table_safe(c.message_template)
            lines.append(
                f"| `{c.signature[:8]}` | {sub} | {c.level} | {c.count} | "
                f"{_table_safe(reason, 40)} | {msg} |"
            )
        lines.append("")
        lines.append("</details>")
        lines.append("")

    return "\n".join(lines)


def render_index(existing: list[str], new_entry: str) -> str:
    entries = sorted(set(existing + [new_entry]), reverse=True)
    lines = ["# Satellite logs review — index", ""]
    for e in entries:
        lines.append(f"- [{e}]({e})")
    lines.append("")
    return "\n".join(lines)


# ---------- GitHub App push ----------

_GITHUB_RETRY_ATTEMPTS = 3
_GITHUB_RETRY_DELAYS = [1, 2, 4]


def _github_request(method: str, url: str, **kwargs) -> requests.Response:
    """Wrap a GitHub API call with simple exponential-backoff retry on 5xx."""
    for attempt, delay in enumerate((*_GITHUB_RETRY_DELAYS, None)):
        r = requests.request(method, url, **kwargs)
        if r.status_code < 500:
            return r
        if delay is None:
            break
        log.warning(
            "GitHub API %s %s returned %d (attempt %d/%d), retrying in %ds",
            method, url, r.status_code, attempt + 1, _GITHUB_RETRY_ATTEMPTS, delay,
        )
        time.sleep(delay)
    return r


def github_installation_token(cfg: Config) -> str:
    now = int(time.time())
    payload = {"iat": now - 60, "exp": now + 9 * 60, "iss": cfg.github_app_id}
    app_jwt = jwt.encode(payload, cfg.github_private_key, algorithm="RS256")
    r = _github_request(
        "POST",
        f"https://api.github.com/app/installations/{cfg.github_installation_id}/access_tokens",
        headers={
            "Authorization": f"Bearer {app_jwt}",
            "Accept": "application/vnd.github+json",
        },
        timeout=30,
    )
    r.raise_for_status()
    return r.json()["token"]


def github_put_file(
    token: str, repo: str, branch: str, path: str, content: str, message: str
) -> None:
    api = f"https://api.github.com/repos/{repo}/contents/{path}"
    headers = {
        "Authorization": f"Bearer {token}",
        "Accept": "application/vnd.github+json",
    }
    existing_sha: str | None = None
    r = _github_request("GET", api, params={"ref": branch}, headers=headers, timeout=30)
    if r.status_code == 200:
        existing_sha = r.json().get("sha")
    elif r.status_code != 404:
        r.raise_for_status()

    body: dict = {
        "message": message,
        "content": base64.b64encode(content.encode("utf-8")).decode("ascii"),
        "branch": branch,
    }
    if existing_sha:
        body["sha"] = existing_sha
    r = _github_request("PUT", api, headers=headers, json=body, timeout=30)
    r.raise_for_status()


def github_list_reports(token: str, repo: str, branch: str, directory: str) -> list[str]:
    r = _github_request(
        "GET",
        f"https://api.github.com/repos/{repo}/contents/{directory}",
        params={"ref": branch},
        headers={"Authorization": f"Bearer {token}", "Accept": "application/vnd.github+json"},
        timeout=30,
    )
    if r.status_code == 404:
        return []
    r.raise_for_status()
    return [
        item["name"]
        for item in r.json()
        if item.get("type") == "file" and item.get("name", "").endswith(".md")
        and item.get("name") != "_index.md"
    ]


def github_issue_exists(
    token: str,
    repo: str,
    group_hash: str,
    cluster_sigs: list[str] | None = None,
) -> str | None:
    """Return the matching label (``log-bug:<hash>`` or ``log-cluster:<sig8>``)
    of an existing duplicate issue, or ``None`` if no duplicate is found.

    The bug-group label ``log-bug:<group_hash>`` is the primary dedup key.
    For backwards compatibility with issues filed by older versions of the
    reviewer (before the bug-group concept existed) — and to defend against
    future cluster-signature drift if we change the normalization rules
    again — we also probe each constituent cluster's ``log-cluster:<sig8>``
    label. Either match counts as a duplicate.

    Both queries restrict to the last 7 days (open or recently closed) so a
    cluster that was triaged and closed long ago can re-file if it recurs.
    """
    cutoff = (
        dt.datetime.now(dt.timezone.utc) - dt.timedelta(days=7)
    ).isoformat(timespec="seconds")
    candidate_labels = [f"log-bug:{group_hash}"]
    for sig in cluster_sigs or []:
        candidate_labels.append(f"log-cluster:{sig[:8]}")
    headers = {"Authorization": f"Bearer {token}", "Accept": "application/vnd.github+json"}
    for label in candidate_labels:
        r = _github_request(
            "GET",
            f"https://api.github.com/repos/{repo}/issues",
            params={"labels": label, "state": "all", "since": cutoff, "per_page": "5"},
            headers=headers,
            timeout=30,
        )
        r.raise_for_status()
        if r.json():
            return label
    return None


def _format_cluster_section(cluster: Cluster) -> str:
    """Render a per-cluster block inside a multi-cluster issue body."""
    parts = [
        f"### Cluster `{cluster.signature[:8]}` — count={cluster.count}",
        "",
        f"- **logger**: `{cluster.logger}`",
        f"- **level**: `{cluster.level}`",
        f"- **container**: `{cluster.container}`",
    ]
    if cluster.samples:
        parts.append("")
        parts.append("<details><summary>Sanitized sample (copy-paste into your debugging agent)</summary>")
        parts.append("")
        sample = sanitize_for_report(
            json.dumps(cluster.samples[0].get("payload", {}), ensure_ascii=False, indent=2)
        )
        parts.append("```json")
        parts.append(sample[:3000])
        parts.append("```")
        parts.append("</details>")
    return "\n".join(parts)


def github_create_issue(
    token: str,
    repo: str,
    group_hash: str,
    clusters: list[Cluster],
    analyses: dict[str, dict],
    report_path: str,
) -> str:
    """Open a GitHub issue for a bug group. Returns the issue URL.

    A bug group may contain multiple log-site clusters that share a root
    cause (e.g. four functions in metabase/changestream all firing during a
    single Spanner outage). The primary cluster — the one with the highest
    count — drives the title, hypothesis, and next steps. Each constituent
    cluster gets its own section in the body so the developer can see the
    full call-site diversity behind the bug.
    """
    primary = clusters[0]
    primary_analysis = analyses.get(primary.signature, {})
    title = primary_analysis.get("summary") or primary.message_template[:100]
    hypothesis = primary_analysis.get("hypothesis", "")
    next_steps = primary_analysis.get("next_steps", [])
    steps_md = "\n".join(f"- {s}" for s in next_steps)
    subsystem = subsystem_for_logger(primary.logger, primary.message_template)
    report_link = f"https://github.com/{repo}/blob/main/{report_path}"
    combined_count = sum(c.count for c in clusters)

    cluster_blocks = "\n\n".join(_format_cluster_section(c) for c in clusters)

    body_parts = [
        f"**Bug group**: `{group_hash}`",
        f"**Subsystem**: `{subsystem}`",
        f"**Clusters in group**: {len(clusters)}",
        f"**Combined occurrences (this run)**: {combined_count}",
        "",
        "## Hypothesis",
        "",
        hypothesis,
        "",
        "## Suggested next steps",
        "",
        steps_md,
        "",
        "## Clusters",
        "",
        cluster_blocks,
        "",
        "## Report",
        "",
        f"[Daily report]({report_link})",
    ]
    body = "\n".join(body_parts)

    labels = ["satellite-log", "auto-triage", f"log-bug:{group_hash}"]
    # Add per-cluster labels for cross-search via the existing log-cluster tag.
    for c in clusters:
        labels.append(f"log-cluster:{c.signature[:8]}")

    r = _github_request(
        "POST",
        f"https://api.github.com/repos/{repo}/issues",
        headers={"Authorization": f"Bearer {token}", "Accept": "application/vnd.github+json"},
        json={"title": sanitize_for_report(title), "body": sanitize_for_report(body), "labels": labels},
        timeout=30,
    )
    r.raise_for_status()
    return r.json()["html_url"]


def open_issues(
    cfg: Config,
    token: str,
    candidates: list[Cluster],
    analyses: dict[str, dict],
    new_signatures: set[str],
    benign: dict[str, str],
    suppressed_sigs: set[str],
    run_date: dt.date,
) -> None:
    """Open GitHub issues for significant NEW bug groups, respecting rate limits.

    Filters benign and suppressed clusters out before grouping, so a group
    only contains real bugs. A group qualifies for an issue if any
    constituent cluster's signature is in ``new_signatures`` and the
    combined count meets ``cfg.issue_threshold``.
    """
    report_path = f"{cfg.report_dir}/{run_date.isoformat()}.md"

    # Drop clusters we don't want issues for (benign, suppressed, no analysis).
    actionable: list[Cluster] = []
    for c in candidates:
        if c.signature in benign:
            log.info(
                "skipping benign cluster %s (count=%d, reason=%s)",
                c.signature, c.count, benign[c.signature],
            )
            continue
        if c.signature in suppressed_sigs:
            log.info("skipping suppressed cluster %s (count=%d)", c.signature, c.count)
            continue
        if not analyses.get(c.signature, {}).get("summary"):
            continue
        actionable.append(c)

    groups = group_clusters(actionable)
    # Sort groups by combined count desc so the loudest bugs file first under
    # the max_issues_per_run cap.
    sorted_groups = sorted(
        groups.items(),
        key=lambda kv: -sum(c.count for c in kv[1]),
    )

    opened = 0
    for group_hash, group in sorted_groups:
        if opened >= cfg.max_issues_per_run:
            break
        combined_count = sum(c.count for c in group)
        if combined_count < cfg.issue_threshold:
            continue
        # Group must include at least one new cluster — otherwise it's a
        # known bug whose signatures have all been seen before.
        if not any(c.signature in new_signatures for c in group):
            continue
        primary = group[0]
        if cfg.dry_run:
            log.info(
                "DRY_RUN=true — would open issue for bug group %s "
                "covering %d clusters (combined count=%d): %s",
                group_hash, len(group), combined_count,
                analyses.get(primary.signature, {}).get("summary", ""),
            )
            opened += 1
            continue
        try:
            cluster_sigs = [c.signature for c in group]
            existing_label = github_issue_exists(
                token, cfg.github_repo, group_hash, cluster_sigs,
            )
            if existing_label:
                log.info(
                    "issue already open for bug group %s under label %s, skipping (covered %d clusters)",
                    group_hash, existing_label, len(group),
                )
                continue
            url = github_create_issue(
                token, cfg.github_repo, group_hash, group, analyses, report_path,
            )
            log.info(
                "opened issue for bug group %s covering %d clusters (combined count=%d): %s",
                group_hash, len(group), combined_count, url,
            )
            opened += 1
        except Exception as exc:
            log.warning(
                "failed to open issue for bug group %s: %s", group_hash, exc,
            )


def publish(cfg: Config, run_date: dt.date, report_md: str) -> None:
    if cfg.dry_run:
        blob_name = f"dry-run/{run_date.isoformat()}.md"
        storage.Client(project=cfg.project).bucket(cfg.state_bucket).blob(
            blob_name
        ).upload_from_string(report_md, content_type="text/markdown")
        log.info(
            "DRY_RUN=true \u2014 wrote report to gs://%s/%s (%d bytes)",
            cfg.state_bucket, blob_name, len(report_md),
        )
        return
    token = github_installation_token(cfg)
    report_name = f"{run_date.isoformat()}.md"
    report_path = f"{cfg.report_dir}/{report_name}"
    github_put_file(
        token, cfg.github_repo, cfg.github_branch, report_path, report_md,
        f"satellite/log-reviewer: daily report {run_date.isoformat()}",
    )
    existing = github_list_reports(token, cfg.github_repo, cfg.github_branch, cfg.report_dir)
    index_md = render_index(existing, report_name)
    github_put_file(
        token, cfg.github_repo, cfg.github_branch, f"{cfg.report_dir}/_index.md", index_md,
        f"satellite/log-reviewer: update index {run_date.isoformat()}",
    )
    log.info("published %s", report_path)


# ---------- entrypoint ----------


def main() -> int:
    cfg = Config.from_env()
    run_ts = dt.datetime.now(dt.timezone.utc).isoformat()
    run_date = dt.datetime.now(dt.timezone.utc).date()

    vertex_init(project=cfg.project, location=cfg.region)
    model = GenerativeModel(cfg.model)

    exclude_filters = load_filters(cfg.filters_path)
    suppressed_sigs = load_suppress(cfg.suppress_path)
    benign_patterns = load_known_benign(cfg.context_path)
    log.info(
        "loaded %d known-benign patterns, %d suppressed signatures",
        len(benign_patterns), len(suppressed_sigs),
    )
    filter_str = build_filter(cfg, exclude_filters)
    log.info("filter: %s", filter_str)

    entries = fetch_logs(cfg, filter_str)
    clusters = cluster_entries(entries, cfg.max_entries_per_cluster)

    state_full = load_state(cfg)
    if cfg.state_reset:
        log.info("STATE_RESET=true \u2014 treating state as empty for classification")
        state_for_classify: dict = {"clusters": {}}
    else:
        state_for_classify = state_full

    new_signatures = classify_new(clusters, state_for_classify)
    log.info(
        "clusters this run: %d total, %d new, %d previously seen",
        len(clusters), len(new_signatures), len(clusters) - len(new_signatures),
    )

    # Match clusters against known-benign patterns (signature -> reason).
    benign: dict[str, str] = {}
    for sig, c in clusters.items():
        reason = match_known_benign(c.message_template, benign_patterns)
        if reason:
            benign[sig] = reason
    log.info("matched %d clusters as known-benign", len(benign))

    # Build analyses dict: cached for ongoing, fresh Gemini for new (bounded).
    analyses: dict[str, dict] = {}
    for sig in clusters:
        cached = cached_analysis(state_for_classify, sig)
        if cached:
            analyses[sig] = cached

    # Candidates for fresh Gemini analysis: NEW, non-benign, non-suppressed,
    # bounded to max_new_clusters_to_analyze. Skip benign \u2014 Gemini analysis
    # on already-understood patterns is wasted spend.
    fresh_candidates = sorted(
        (c for sig, c in clusters.items()
         if sig in new_signatures
         and sig not in benign
         and sig not in suppressed_sigs),
        key=lambda c: -c.count,
    )[: cfg.max_new_clusters_to_analyze]

    codebase_context = cfg.context_path.read_text() if cfg.context_path.exists() else ""
    analyzer = _Analyzer(cfg, model, codebase_context)
    fresh_count = 0
    for c in fresh_candidates:
        try:
            analyses[c.signature] = analyzer.analyze(c)
            fresh_count += 1
        except GeminiUnavailable as exc:
            log.error("aborting run: %s", exc)
            return 1
    log.info(
        "analysis: %d fresh from Gemini, %d cached from state, %d clusters total",
        fresh_count, len(analyses) - fresh_count, len(clusters),
    )

    report_md = render_report(
        cfg=cfg,
        run_date=run_date,
        total_entries=len(entries),
        clusters_this_run=list(clusters.values()),
        analyses=analyses,
        new_signatures=new_signatures,
        benign=benign,
        suppressed_sigs=suppressed_sigs,
        state=state_for_classify,
    )

    publish(cfg, run_date, report_md)

    issue_token = github_installation_token(cfg) if not cfg.dry_run else ""
    open_issues(
        cfg, issue_token,
        candidates=list(clusters.values()),
        analyses=analyses,
        new_signatures=new_signatures,
        benign=benign,
        suppressed_sigs=suppressed_sigs,
        run_date=run_date,
    )

    # only persist state AFTER a successful publish so a failed run can be retried.
    # Always update against the live (non-reset) state \u2014 STATE_RESET only affects
    # classification, not what we persist forward.
    state_full = update_state(state_full, clusters, analyses, run_ts)
    save_state(cfg, state_full)
    log.info("run complete")
    return 0


if __name__ == "__main__":
    try:
        sys.exit(main())
    except Exception:
        log.exception("run failed")
        sys.exit(1)
