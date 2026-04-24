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
            model=os.environ.get("GEMINI_MODEL", "gemini-3.1-pro"),
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
            github_repo=os.environ.get("GITHUB_REPO", "storj/storj"),
            github_branch=os.environ.get(
                "GITHUB_BRANCH", "claude/review-satellite-logs-EGmLz"
            ),
            report_dir=os.environ.get("REPORT_DIR", "reports/satellite-logs"),
            filters_path=here / "filters.yaml",
            suppress_path=here / "suppress.yaml",
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
    max_entries = int(os.environ.get("MAX_ENTRIES", "50000"))
    sleep_between_pages = float(os.environ.get("READ_SLEEP_S", "1.5"))

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
_BASE64ISH_RE = re.compile(r"\b[A-Za-z0-9+/=]{24,}\b")
# Stripe-style opaque IDs: req_/cus_/sub_/in_/ii_/ch_/prod_/price_/acct_ + 10+ chars
_OPAQUE_ID_RE = re.compile(r"\b(req|cus|sub|in|ii|ch|prod|price|acct|pi|pm|tok|txn|re)_[A-Za-z0-9]{8,}\b")

# PII redaction (applied before the sample goes into the report)
_EMAIL_RE = re.compile(r"\b[\w.+-]+@[\w-]+\.[\w.-]+\b")
_BEARER_RE = re.compile(r"(?i)(bearer\s+)[A-Za-z0-9._-]+")
_SIGNED_URL_QS_RE = re.compile(r"(?i)([?&](?:signature|token|key|sig)=)[^&\s]+")


def normalize_for_signature(s: str) -> str:
    s = _UUID_RE.sub("<UUID>", s)
    s = _TS_RE.sub("<TS>", s)
    s = _IP_RE.sub("<IP>", s)
    s = _BASE64ISH_RE.sub("<B64>", s)
    s = _OPAQUE_ID_RE.sub("<ID>", s)
    s = _HEX_RE.sub("<HEX>", s)
    s = _NUM_RE.sub("<N>", s)
    return s.strip()


def sanitize_for_report(s: str) -> str:
    s = _EMAIL_RE.sub("<redacted:email>", s)
    s = _BEARER_RE.sub(r"\1<redacted:token>", s)
    s = _SIGNED_URL_QS_RE.sub(r"\1<redacted>", s)
    return s


def entry_signature(entry: dict) -> tuple[str, dict]:
    """Return (signature, enriched metadata) for clustering."""
    p = entry.get("payload") or {}
    level = p.get("L") or entry.get("severity") or "UNKNOWN"
    logger_name = p.get("N") or "<unknown-logger>"
    message = p.get("M") or p.get("message") or ""
    err = p.get("error") or p.get("err") or ""

    raw_key = f"{logger_name}|{level}|{normalize_for_signature(str(message))}|{normalize_for_signature(str(err))}"
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


def update_state(state: dict, clusters: dict[str, Cluster], run_ts: str) -> dict:
    """Record clusters seen this run and evict stale entries."""
    known = state.get("clusters", {})
    for sig, c in clusters.items():
        existing = known.get(sig)
        if existing is None:
            known[sig] = {
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
    state["clusters"] = known
    state["last_run"] = run_ts
    return state


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


# ---------- Gemini hypothesis ----------


HYPOTHESIS_PROMPT = """You are a senior Storj satellite SRE triaging a newly-seen log cluster.

Given the logger name, level, message template, and a few sanitized sample
entries, produce:
1. A one-line summary (<=100 chars).
2. A short hypothesis about the likely root cause (2-4 sentences). If the
   logger name looks like a Go package path, you may reference which
   satellite subsystem it probably belongs to. Do NOT invent file paths or
   function names you cannot derive from the logger name.
3. 2-3 concrete investigation steps.

Respond ONLY as JSON:
{{
  "summary": "...",
  "hypothesis": "...",
  "next_steps": ["...", "..."]
}}

Cluster:
- logger: {logger}
- level: {level}
- container: {container}
- message_template: {message_template}
- error_template: {error_template}
- occurrences_this_run: {count}

Sanitized samples:
{samples}
"""


def analyze_cluster(cfg: Config, model: GenerativeModel, cluster: Cluster) -> dict:
    samples_text = "\n".join(
        "- " + sanitize_for_report(json.dumps(s.get("payload", {}), ensure_ascii=False))[:1000]
        for s in cluster.samples[:5]
    )
    prompt = HYPOTHESIS_PROMPT.format(
        logger=cluster.logger,
        level=cluster.level,
        container=cluster.container,
        message_template=cluster.message_template,
        error_template=cluster.error_template,
        count=cluster.count,
        samples=samples_text,
    )
    try:
        resp = model.generate_content(
            prompt,
            generation_config={"temperature": 0.2, "response_mime_type": "application/json"},
        )
        return json.loads(resp.text)
    except Exception as exc:
        log.warning("gemini analysis failed for %s: %s", cluster.signature, exc)
        return {
            "summary": cluster.message_template[:100],
            "hypothesis": f"(automatic analysis failed: {exc})",
            "next_steps": ["review sample logs manually"],
        }


# ----------  envirorendering ----------


def classify(
    clusters: dict[str, Cluster],
    state: dict,
    suppress: set[str],
) -> tuple[list[Cluster], list[Cluster], list[dict], list[Cluster]]:
    """Split clusters into (new, ongoing, silent, suppressed)."""
    known = state.get("clusters", {})
    new: list[Cluster] = []
    ongoing: list[Cluster] = []
    suppressed: list[Cluster] = []
    for sig, c in clusters.items():
        if sig in suppress:
            suppressed.append(c)
        elif sig in known:
            ongoing.append(c)
        else:
            new.append(c)
    # silent = in state but not in this run
    seen_this_run = set(clusters.keys())
    silent = [
        v for sig, v in known.items()
        if sig not in seen_this_run and sig not in suppress
    ]
    new.sort(key=lambda c: c.count, reverse=True)
    ongoing.sort(key=lambda c: c.count, reverse=True)
    return new, ongoing, silent, suppressed


def render_report(
    cfg: Config,
    run_date: dt.date,
    total_entries: int,
    new: list[Cluster],
    new_analyses: dict[str, dict],
    ongoing: list[Cluster],
    silent: list[dict],
    suppressed: list[Cluster],
) -> str:
    lines: list[str] = []
    lines.append(f"# Satellite logs review — {run_date.isoformat()}")
    lines.append("")
    lines.append(
        f"- project: `{cfg.project}`  window: {cfg.window_hours}h  "
        f"entries scanned: {total_entries}"
    )
    lines.append(
        f"- clusters: new={len(new)} ongoing={len(ongoing)} "
        f"silent={len(silent)} suppressed={len(suppressed)}"
    )
    lines.append("")

    lines.append(f"## New anomalies ({len(new)})")
    lines.append("")
    if not new:
        lines.append("_None._")
        lines.append("")
    for c in new:
        a = new_analyses.get(c.signature, {})
        summary = a.get("summary") or c.message_template[:100]
        lines.append(f"### `{c.signature}` — {sanitize_for_report(summary)}")
        lines.append("")
        lines.append(f"- **logger**: `{c.logger}`")
        lines.append(f"- **level**: `{c.level}`")
        lines.append(f"- **container**: `{c.container}`")
        lines.append(f"- **count (this run)**: {c.count}")
        lines.append(f"- **first/last seen (this run)**: {c.first_seen_in_run} / {c.last_seen_in_run}")
        lines.append("")
        if a.get("hypothesis"):
            lines.append("**Hypothesis.** " + sanitize_for_report(a["hypothesis"]))
            lines.append("")
        if a.get("next_steps"):
            lines.append("**Next steps:**")
            for step in a["next_steps"]:
                lines.append(f"- {sanitize_for_report(step)}")
            lines.append("")
        lines.append("<details><summary>Sanitized samples</summary>")
        lines.append("")
        lines.append("```json")
        for s in c.samples[:5]:
            lines.append(sanitize_for_report(json.dumps(s.get("payload", {}), ensure_ascii=False))[:2000])
        lines.append("```")
        lines.append("</details>")
        lines.append("")

    lines.append(f"## Ongoing anomalies ({len(ongoing)})")
    lines.append("")
    if not ongoing:
        lines.append("_None._")
    else:
        lines.append("| signature | logger | level | count | message |")
        lines.append("|---|---|---|---:|---|")
        for c in ongoing:
            msg = sanitize_for_report(c.message_template)[:120].replace("|", "\\|")
            lines.append(
                f"| `{c.signature}` | `{c.logger}` | {c.level} | {c.count} | {msg} |"
            )
    lines.append("")

    lines.append(f"## Went silent ({len(silent)})")
    lines.append("")
    if not silent:
        lines.append("_None._")
    else:
        lines.append("| signature | logger | last seen ever | total count |")
        lines.append("|---|---|---|---:|")
        for v in silent[:50]:
            lines.append(
                f"| `{v['signature']}` | `{v.get('logger','?')}` "
                f"| {v.get('last_seen_ever','?')} | {v.get('total_count','?')} |"
            )
    lines.append("")

    lines.append(f"## Suppressed ({len(suppressed)})")
    lines.append("")
    if not suppressed:
        lines.append("_None._")
    else:
        for c in suppressed:
            lines.append(f"- `{c.signature}` — {sanitize_for_report(c.message_template)[:120]} (count={c.count})")
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


def github_installation_token(cfg: Config) -> str:
    now = int(time.time())
    payload = {"iat": now - 60, "exp": now + 9 * 60, "iss": cfg.github_app_id}
    app_jwt = jwt.encode(payload, cfg.github_private_key, algorithm="RS256")
    r = requests.post(
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
    r = requests.get(api, params={"ref": branch}, headers=headers, timeout=30)
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
    r = requests.put(api, headers=headers, data=json.dumps(body), timeout=30)
    r.raise_for_status()


def github_list_reports(token: str, repo: str, branch: str, directory: str) -> list[str]:
    r = requests.get(
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
    suppress = load_suppress(cfg.suppress_path)
    filter_str = build_filter(cfg, exclude_filters)
    log.info("filter: %s", filter_str)

    entries = fetch_logs(cfg, filter_str)
    clusters = cluster_entries(entries, cfg.max_entries_per_cluster)

    state = load_state(cfg)
    new, ongoing, silent, suppressed = classify(clusters, state, suppress)
    log.info(
        "cluster summary: new=%d ongoing=%d silent=%d suppressed=%d",
        len(new), len(ongoing), len(silent), len(suppressed),
    )
    for c in sorted(new, key=lambda x: -x.total_count)[:30]:
        preview = (c.sample_message[:120] + "\u2026") if len(c.sample_message) > 120 else c.sample_message
        log.info("new cluster %s count=%d level=%s msg=%r", c.signature, c.total_count, c.level, preview)

    # analyze only NEW clusters, bounded
    new_to_analyze = new[: cfg.max_new_clusters_to_analyze]
    analyses: dict[str, dict] = {}
    for c in new_to_analyze:
        analyses[c.signature] = analyze_cluster(cfg, model, c)

    report_md = render_report(
        cfg, run_date, len(entries), new, analyses, ongoing, silent, suppressed
    )

    publish(cfg, run_date, report_md)

    # only persist state AFTER a successful publish so a failed run can be retried
    update_state(state, clusters, run_ts)
    save_state(cfg, state)
    log.info("run complete")
    return 0


if __name__ == "__main__":
    try:
        sys.exit(main())
    except Exception:
        log.exception("run failed")
        sys.exit(1)
