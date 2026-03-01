#!/usr/bin/env python3
import argparse
import json
import re
import subprocess
import sys
import time
from collections import defaultdict
from dataclasses import dataclass
from datetime import date
from pathlib import Path
from typing import Any, Dict, List, Optional

try:
    import yaml
except ImportError as exc:
    raise SystemExit("PyYAML is required. Install with: pip install pyyaml") from exc

HTML_TAG_RE = re.compile(r"<\s*/?\s*(?:div|span|p|a|img|script|style|link|br|hr|table|tr|td|th|ul|ol|li|h[1-6]|form|input|button|section|article|nav|header|footer|iframe|video|audio|source|meta)\b[^>]*>", re.IGNORECASE)
ALLOWED_STATUSES = {"ok", "fail"}


@dataclass
class RunResult:
    run_index: int
    elapsed_seconds: float
    command: List[str]
    exit_code: int
    stdout: str
    stderr: str
    parse_ok: bool
    response: Optional[Dict[str, Any]]
    checks: Dict[str, Any]
    score: int
    max_score: int
    passed: bool
    reasons: List[str]


def load_manifest(path: Path) -> Dict[str, Any]:
    with path.open("r", encoding="utf-8") as f:
        data = yaml.safe_load(f)
    if not isinstance(data, dict) or "tests" not in data or not isinstance(data["tests"], list):
        raise ValueError("Manifest must contain top-level 'tests' list")

    for i, tc in enumerate(data["tests"], start=1):
        if not isinstance(tc, dict):
            raise ValueError(f"tests[{i}] must be a mapping")
        if "url" not in tc:
            raise ValueError(f"tests[{i}] is missing required field 'url'")

        expect = tc.get("expect", {}) or {}
        status = str(expect.get("status", "ok")).strip().lower()
        if status not in ALLOWED_STATUSES:
            raise ValueError(
                f"tests[{i}] has invalid expect.status='{status}'. Allowed values: {sorted(ALLOWED_STATUSES)}"
            )

    return data


def run_ws(url: str, ws_binary: str, command_timeout: float) -> Dict[str, Any]:
    cmd = [ws_binary, "read", url, "--no-cache", "--format", "json"]
    start = time.perf_counter()
    timed_out = False
    try:
        proc = subprocess.run(cmd, capture_output=True, text=True, timeout=command_timeout)
    except subprocess.TimeoutExpired as exc:
        elapsed = time.perf_counter() - start
        timed_out = True
        return {
            "command": cmd,
            "elapsed_seconds": elapsed,
            "exit_code": 124,
            "stdout": (exc.stdout or "").strip() if isinstance(exc.stdout, str) else "",
            "stderr": f"timeout after {command_timeout}s",
            "parse_ok": False,
            "response": None,
            "timed_out": timed_out,
        }

    elapsed = time.perf_counter() - start

    stdout = (proc.stdout or "").strip()
    stderr = (proc.stderr or "").strip()

    response = None
    parse_ok = False
    if proc.returncode == 0 and stdout:
        try:
            response = json.loads(stdout)
            parse_ok = True
        except json.JSONDecodeError:
            parse_ok = False

    return {
        "command": cmd,
        "elapsed_seconds": elapsed,
        "exit_code": proc.returncode,
        "stdout": stdout,
        "stderr": stderr,
        "parse_ok": parse_ok,
        "response": response,
        "timed_out": timed_out,
    }


def evaluate_run(tc: Dict[str, Any], raw: Dict[str, Any], run_index: int) -> RunResult:
    expect = tc.get("expect", {}) or {}
    expected_status = str(expect.get("status", "ok")).strip().lower()

    response = raw.get("response") if raw.get("parse_ok") else None
    title = str((response or {}).get("title", "") or "")
    content = str((response or {}).get("content", "") or "")
    backend = str((response or {}).get("backend", "") or "")

    status_ok = raw["exit_code"] == 0 and raw.get("parse_ok") and len(content.strip()) > 0
    status_match = (expected_status == "ok" and status_ok) or (expected_status == "fail" and not status_ok)

    title_contains = str(expect.get("title_contains", ""))
    min_len = int(expect.get("min_content_length", 0) or 0)
    no_html_tags = bool(expect.get("no_html_tags", False))
    max_time = expect.get("max_time_seconds", None)
    expected_backend = str(expect.get("backend", "")).strip().lower() if "backend" in expect else ""

    if status_ok:
        title_match = title_contains.lower() in title.lower()
        content_length_match = len(content) >= min_len
        clean_content_match = (not no_html_tags) or (HTML_TAG_RE.search(content) is None)
    elif expected_status == "fail":
        # Expected-failure tests don't have meaningful content/title quality checks.
        title_match = True
        content_length_match = True
        clean_content_match = True
    else:
        title_match = False
        content_length_match = False
        clean_content_match = False

    if max_time is None:
        speed_match = True
    else:
        speed_match = raw["elapsed_seconds"] <= float(max_time)

    backend_match = True
    if expected_backend:
        backend_match = status_ok and backend.lower() == expected_backend

    checks = {
        "status_match": status_match,
        "backend_match": backend_match,
        "title_match": title_match,
        "content_length": {
            "actual": len(content) if status_ok else 0,
            "expected_min": min_len,
            "pass": content_length_match,
        },
        "clean_content": {
            "enabled": no_html_tags,
            "pass": clean_content_match,
        },
        "speed": {
            "actual_seconds": round(raw["elapsed_seconds"], 3),
            "expected_max_seconds": max_time,
            "pass": speed_match,
        },
        "expected_status": expected_status,
        "actual_status": "ok" if status_ok else "fail",
        "expected_backend": expected_backend,
        "actual_backend": backend,
        "timed_out": raw.get("timed_out", False),
    }

    score_parts = {
        "title_match": int(title_match),
        "content_length": int(content_length_match),
        "clean_content": int(clean_content_match),
        "speed": int(speed_match),
    }
    score = sum(score_parts.values())
    max_score = 4

    reasons: List[str] = []
    if not status_match:
        reasons.append(f"status={checks['actual_status']} expected={expected_status}")
    if expected_backend and not backend_match:
        reasons.append(f"backend={backend or '<none>'} expected={expected_backend}")
    if expected_status == "ok" and not title_match:
        reasons.append(f"title mismatch (expected contains '{title_contains}')")
    if expected_status == "ok" and not content_length_match:
        reasons.append(f"content_length={len(content) if status_ok else 0} (expected >={min_len})")
    if no_html_tags and expected_status == "ok" and not clean_content_match:
        reasons.append("content contains HTML tags")
    if max_time is not None and not speed_match:
        reasons.append(f"time={raw['elapsed_seconds']:.2f}s (expected <={float(max_time):.2f}s)")
    if raw.get("timed_out"):
        reasons.append("command timed out")

    passed = status_match and backend_match
    if expected_status == "ok":
        passed = passed and title_match and content_length_match and clean_content_match and speed_match
    else:
        passed = passed and speed_match

    return RunResult(
        run_index=run_index,
        elapsed_seconds=raw["elapsed_seconds"],
        command=raw["command"],
        exit_code=raw["exit_code"],
        stdout=raw["stdout"],
        stderr=raw["stderr"],
        parse_ok=raw["parse_ok"],
        response=response,
        checks=checks,
        score=score,
        max_score=max_score,
        passed=passed,
        reasons=reasons,
    )


def finalize_case(tc: Dict[str, Any], runs: List[RunResult]) -> Dict[str, Any]:
    if not runs:
        raise ValueError(f"No runs executed for {tc.get('url', '<unknown>')}")

    run_statuses = ["ok" if r.passed else "fail" for r in runs]
    if all(r.passed for r in runs):
        final_status = "pass"
    elif any(r.passed for r in runs):
        final_status = "flaky"
    else:
        final_status = "fail"

    # Penalize instability and worst-case behavior in aggregate score.
    case_score = min(r.score for r in runs)
    case_max = runs[0].max_score

    return {
        "name": tc.get("name") or tc["url"],
        "url": tc["url"],
        "category": tc.get("category", "uncategorized"),
        "expect": tc.get("expect", {}),
        "status": final_status,
        "run_statuses": run_statuses,
        "score": case_score,
        "max_score": case_max,
        "runs": [
            {
                "run_index": r.run_index,
                "elapsed_seconds": round(r.elapsed_seconds, 3),
                "exit_code": r.exit_code,
                "parse_ok": r.parse_ok,
                "checks": r.checks,
                "score": r.score,
                "max_score": r.max_score,
                "passed": r.passed,
                "reasons": r.reasons,
                "stderr": r.stderr,
            }
            for r in runs
        ],
    }


def build_summary(report: Dict[str, Any]) -> str:
    lines: List[str] = []
    lines.append("=== WebScout Test Report ===")
    lines.append(f"Tier: {report['tier']}")
    lines.append(f"Date: {report['date']}")
    lines.append(
        f"Total: {report['total']} | Pass: {report['pass']} | Fail: {report['fail']} | Flaky: {report['flaky']}"
    )
    pct = (report["score"] / report["score_max"] * 100.0) if report["score_max"] else 0.0
    lines.append(f"Score: {report['score']}/{report['score_max']} ({pct:.1f}%)")
    lines.append("")
    lines.append("FAILURES:")

    if not report["failures"]:
        lines.append("  (none)")
    else:
        for f in report["failures"]:
            lines.append(f"  {f}")

    lines.append("")
    lines.append("BY CATEGORY:")
    for category, stat in sorted(report["by_category"].items()):
        cat_pct = (stat["score"] / stat["score_max"] * 100.0) if stat["score_max"] else 0.0
        lines.append(f"  {category:<12} {stat['score']}/{stat['score_max']} ({cat_pct:.0f}%)")

    return "\n".join(lines)


def main() -> int:
    parser = argparse.ArgumentParser(description="WebScout tier test harness")
    parser.add_argument("manifest", help="Path to YAML test manifest")
    parser.add_argument("--results", default="tests/results.json", help="Output JSON report path")
    parser.add_argument("--runs", type=int, default=2, help="Runs per URL (default: 2)")
    parser.add_argument("--ws", default="./ws", help="Path to ws executable")
    parser.add_argument(
        "--command-timeout",
        type=float,
        default=45.0,
        help="Per ws invocation timeout in seconds (default: 45)",
    )
    args = parser.parse_args()

    if args.runs < 1:
        print("--runs must be >= 1", file=sys.stderr)
        return 2
    if args.command_timeout <= 0:
        print("--command-timeout must be > 0", file=sys.stderr)
        return 2

    manifest_path = Path(args.manifest)
    results_path = Path(args.results)

    try:
        manifest = load_manifest(manifest_path)
    except Exception as exc:
        print(f"Failed to load manifest {manifest_path}: {exc}", file=sys.stderr)
        return 2

    cases = manifest["tests"]
    tier = manifest_path.stem
    report_cases: List[Dict[str, Any]] = []

    for idx, tc in enumerate(cases, start=1):
        if "url" not in tc:
            print(f"Skipping test {idx}: missing url", file=sys.stderr)
            continue
        runs: List[RunResult] = []
        for run_idx in range(1, args.runs + 1):
            raw = run_ws(tc["url"], args.ws, args.command_timeout)
            run = evaluate_run(tc, raw, run_idx)
            runs.append(run)
        report_cases.append(finalize_case(tc, runs))

    total = len(report_cases)
    pass_count = sum(1 for c in report_cases if c["status"] == "pass")
    fail_count = sum(1 for c in report_cases if c["status"] == "fail")
    flaky_count = sum(1 for c in report_cases if c["status"] == "flaky")
    total_score = sum(c["score"] for c in report_cases)
    total_max = sum(c["max_score"] for c in report_cases)

    by_category: Dict[str, Dict[str, int]] = defaultdict(lambda: {"score": 0, "score_max": 0})
    failures: List[str] = []

    for c in report_cases:
        cat = c["category"]
        by_category[cat]["score"] += c["score"]
        by_category[cat]["score_max"] += c["max_score"]

        if c["status"] == "fail":
            r = c["runs"][0]
            reason = "; ".join(r.get("reasons") or ["unknown"])
            failures.append(f"[FAIL] {c['name']} ({c['url']}): {reason}")
        elif c["status"] == "flaky":
            run_bits = [f"run{r['run_index']}={'ok' if r['passed'] else 'fail'}" for r in c["runs"]]
            reason = "inconsistent run outcomes"
            for r in c["runs"]:
                if not r["passed"] and r.get("reasons"):
                    reason = "; ".join(r["reasons"])
                    break
            failures.append(f"[FLAKY] {c['name']} ({c['url']}): {' '.join(run_bits)} ({reason})")

    report = {
        "tier": tier,
        "date": str(date.today()),
        "total": total,
        "pass": pass_count,
        "fail": fail_count,
        "flaky": flaky_count,
        "score": total_score,
        "score_max": total_max,
        "cases": report_cases,
        "failures": failures,
        "by_category": by_category,
    }

    results_path.parent.mkdir(parents=True, exist_ok=True)
    with results_path.open("w", encoding="utf-8") as f:
        json.dump(report, f, ensure_ascii=False, indent=2)

    summary = build_summary(report)
    print(summary)
    return 0 if fail_count == 0 and flaky_count == 0 else 1


if __name__ == "__main__":
    raise SystemExit(main())
