#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
"""Read existing MLflow feedback; export aggregates, never trace bodies or prose.

A trusted consumer supplies endpoint, experiment and source from its base config.
Only metadata search is called. This does not run a judge or write to MLflow.
"""
import argparse
import base64
from datetime import datetime, timedelta, timezone
import hashlib
import json
import os
from pathlib import Path
import re
import urllib.parse
import urllib.request


def require(ok, code):
    if not ok:
        raise ValueError(code)


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, *args):
        return None


def timestamp(text):
    parsed = datetime.fromisoformat(text.replace("Z", "+00:00"))
    require(parsed.tzinfo is not None, "timestamp_timezone_missing")
    return parsed.astimezone(timezone.utc)


def collect(binding, now, fetch):
    require(binding["schema_version"] == "judge-binding/v1", "invalid_judge_binding")
    require(re.fullmatch(r"[0-9]+", binding["experiment_id"]), "invalid_experiment")
    require(binding["scorer"] == "review_correctly_approved", "unsupported_scorer")
    require(
        type(binding["lookback_days"]) is int and 1 <= binding["lookback_days"] <= 365,
        "invalid_window",
    )
    now = now.replace(microsecond=0)
    start = now - timedelta(days=binding["lookback_days"])
    body = {
        "locations": [
            {
                "type": "MLFLOW_EXPERIMENT",
                "mlflow_experiment": {"experiment_id": binding["experiment_id"]},
            }
        ],
        "max_results": 100,
        "order_by": ["timestamp_ms DESC"],
        "filter": f"feedback.review_correctly_approved IS NOT NULL AND trace.timestamp_ms >= {int(start.timestamp()*1000)} AND trace.timestamp_ms < {int(now.timestamp()*1000)}",
    }
    seen, seen_tokens, rows = set(), set(), []
    passed = invalid = excluded = 0
    latest = None
    complete = False
    for _ in range(50):
        page = fetch(body)
        require(
            isinstance(page, dict)
            and isinstance(page.get("traces"), list)
            and len(page["traces"]) <= 100,
            "invalid_search_page",
        )
        require(
            "next_page_token" not in page or isinstance(page["next_page_token"], str),
            "invalid_page_token",
        )
        for trace in page["traces"]:
            trace_id = trace["trace_id"]
            require(trace_id not in seen, "duplicate_trace")
            seen.add(trace_id)
            requested = timestamp(trace["request_time"])
            require(start <= requested < now, "trace_outside_window")
            require(
                str(trace["trace_location"]["mlflow_experiment"]["experiment_id"])
                == binding["experiment_id"],
                "wrong_experiment",
            )
            matches = [
                a
                for a in trace.get("assessments", [])
                if a.get("assessment_name") == binding["scorer"]
                and a.get("source", {}).get("source_id") == binding["source_id"]
            ]
            if not matches:
                excluded += 1
                continue
            if (
                len(matches) != 1
                or trace.get("state") != "OK"
                or matches[0].get("valid") is not True
                or matches[0].get("source", {}).get("source_type") != "LLM_JUDGE"
            ):
                invalid += 1
                continue
            a = matches[0]
            value = a.get("feedback", {}).get("value")
            if (
                type(value) not in (float, int)
                or value not in (0, 1)
                or a.get("error")
                or a.get("feedback", {}).get("error")
            ):
                invalid += 1
                continue
            updated = timestamp(a["last_update_time"])
            if not requested <= updated <= now:
                invalid += 1
                continue
            passed += int(value)
            latest = max(latest or updated, updated)
            # Only a digest leaves memory; trace IDs and rationale are not exported.
            rows.append([trace_id, a["assessment_id"], value, a["last_update_time"]])
        token = page.get("next_page_token")
        if not token:
            complete = True
            break
        require(token not in seen_tokens, "pagination_cycle")
        seen_tokens.add(token)
        body["page_token"] = token
    fmt = lambda t: t.isoformat(timespec="seconds").replace("+00:00", "Z")
    return {
        "schema_version": "judge-observation/v1",
        "scorer": binding["scorer"],
        "source_id": binding["source_id"],
        "experiment_id": binding["experiment_id"],
        "cohort": "experiment-scored-approvals",
        "window_start": fmt(start),
        "window_end": fmt(now),
        "collected_at": fmt(now),
        "latest_score_at": fmt(latest) if latest else "",
        "passed": passed,
        "total": len(rows),
        "invalid": invalid,
        "excluded_other_sources": excluded,
        "complete": complete,
        "records_sha256": hashlib.sha256(
            json.dumps(sorted(rows), separators=(",", ":")).encode()
        ).hexdigest(),
        "authorization": "none",
        "mutation_permitted": False,
    }


def collect_stable(binding, now, fetch):
    """Two equal bounded scans detect common pagination races, not DB isolation."""
    now = now.replace(microsecond=0)
    first = collect(binding, now, fetch)
    if not first["complete"]:
        return first
    second = collect(binding, now, fetch)
    require(first == second, "judge_population_changed")
    return second


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--binding", required=True)
    p.add_argument("--output", required=True)
    args = p.parse_args()
    binding = json.loads(Path(args.binding).read_text())
    endpoint = binding["endpoint"]
    url = urllib.parse.urlsplit(endpoint)
    require(
        url.scheme == "https"
        and url.hostname
        and not url.username
        and not url.password
        and not url.query
        and not url.fragment
        and url.path in ("", "/"),
        "invalid_endpoint",
    )
    user, password = os.environ.get("MLFLOW_READER_USERNAME"), os.environ.get(
        "MLFLOW_READER_PASSWORD"
    )
    require(user and password, "reader_credentials_missing")
    headers = {
        "Authorization": "Basic "
        + base64.b64encode((user + ":" + password).encode()).decode(),
        "Content-Type": "application/json",
    }
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}), NoRedirect())

    def fetch(body):
        request = urllib.request.Request(
            endpoint.rstrip("/") + "/api/3.0/mlflow/traces/search",
            data=json.dumps(body).encode(),
            headers=headers,
        )
        with opener.open(request, timeout=30) as response:
            raw = response.read(4 * 1024 * 1024 + 1)
        require(len(raw) <= 4 * 1024 * 1024, "response_too_large")
        return json.loads(raw)

    report = collect_stable(binding, datetime.now(timezone.utc), fetch)
    output = Path(args.output)
    # O_EXCL rejects existing files and symlinks; no public-permission window.
    fd = os.open(output, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(fd, "w") as stream:
        stream.write(json.dumps(report, indent=2) + "\n")
    print(
        json.dumps(
            {
                "total": report["total"],
                "passed": report["passed"],
                "invalid": report["invalid"],
                "complete": report["complete"],
            }
        )
    )


if __name__ == "__main__":
    try:
        main()
    except Exception as error:
        reason = str(error)
        raise SystemExit(
            reason
            if re.fullmatch("[a-z0-9_]{1,80}", reason)
            else "judge_collection_failed"
        )
