#!/usr/bin/env python3
"""Validate shared documents and measured logs against pinned official schemas.

This optional check downloads only public, digest-pinned schema/tool archives.
It never reads credentials and never executes a forge operation.
"""

import hashlib
import io
import json
from pathlib import Path
import platform
import subprocess
import sys
import tarfile
import tempfile
import urllib.request

ROOT = Path(__file__).resolve().parents[1]


def digest(raw):
    return hashlib.sha256(raw).hexdigest()


def download(url, expected, limit):
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
    with opener.open(url, timeout=45) as response:
        raw = response.read(limit + 1)
    if len(raw) > limit or digest(raw) != expected:
        raise ValueError("archive_verification_failed")
    return raw


def tools():
    if platform.system() != "Linux" or platform.machine() not in {"x86_64", "amd64"}:
        raise ValueError("schema_bootstrap_requires_linux_amd64")
    lock = json.loads((ROOT / "schema-toolchain.json").read_text())
    cache = ROOT / ".schema-tools"
    if cache.is_symlink():
        raise ValueError("symlink_tool_directory")
    cache.mkdir(exist_ok=True)
    cue = cache / "cue"
    pin = lock["cue"]
    if cue.is_symlink():
        raise ValueError("symlink_validator")
    if not cue.exists() or digest(cue.read_bytes()) != pin["binary_sha256"]:
        raw = download(pin["archive_url"], pin["archive_sha256"], 50 * 1024 * 1024)
        with tarfile.open(fileobj=io.BytesIO(raw)) as archive:
            entries = [x for x in archive.getmembers() if x.name == "cue"]
            if len(entries) != 1 or not entries[0].isfile() or entries[0].size > 100 * 1024 * 1024:
                raise ValueError("invalid_validator_archive")
            binary = archive.extractfile(entries[0]).read()
        if digest(binary) != pin["binary_sha256"]:
            raise ValueError("validator_digest_mismatch")
        cue.write_bytes(binary)
        cue.chmod(0o755)
    schemas = cache / "schema"
    if schemas.is_symlink():
        raise ValueError("symlink_schemas")
    schemas.mkdir(exist_ok=True)
    pin = lock["gemara"]
    if not all((schemas / n).is_file() and not (schemas / n).is_symlink()
               and digest((schemas / n).read_bytes()) == h for n, h in pin["schema_files"].items()):
        raw = download(pin["archive_url"], pin["archive_sha256"], 5 * 1024 * 1024)
        found = set()
        with tarfile.open(fileobj=io.BytesIO(raw)) as archive:
            for member in archive:
                parts = Path(member.name).parts
                if len(parts) != 2 or parts[1] not in pin["schema_files"]:
                    continue
                name = parts[1]
                if name in found or not member.isfile() or member.size > 1024 * 1024:
                    raise ValueError("invalid_schema_archive")
                data = archive.extractfile(member).read()
                target = schemas / name
                if target.is_symlink() or digest(data) != pin["schema_files"][name]:
                    raise ValueError("schema_digest_mismatch")
                target.write_bytes(data)
                found.add(name)
        if found != set(pin["schema_files"]):
            raise ValueError("schema_archive_incomplete")
    return cue, [schemas / name for name in sorted(pin["schema_files"]) if name.endswith(".cue")]


def main():
    if len(sys.argv) != 2:
        raise ValueError("usage_validate_artifacts_results_json")
    cue, schemas = tools()
    with open(sys.argv[1], "rb") as stream:
        raw = stream.read(2 * 1024 * 1024 + 1)
    if len(raw) > 2 * 1024 * 1024:
        raise ValueError("results_oversized")
    results = json.loads(raw)
    if len(results) != 18:
        raise ValueError("incomplete_demo_results")
    inputs = [(ROOT / "policy/catalog.json", "ControlCatalog"), (ROOT / "policy/common.json", "Policy")]
    with tempfile.TemporaryDirectory(prefix="gemara-schema-") as directory:
        for i, item in enumerate(results):
            report = item["report"]
            if report["evidence_kind"] != "synthetic-fixture" or report["mutation_permitted"] or report["authorization"] != "none":
                raise ValueError("unexpected_authority")
            path = Path(directory) / (str(i) + ".json")
            path.write_text(json.dumps(report["evaluation_log"]))
            inputs.append((path, "EvaluationLog"))
        for path, kind in inputs:
            result = subprocess.run([str(cue), "vet", "-c", "-d", "#" + kind, *map(str, schemas), str(path)],
                                    cwd=ROOT, env={"PATH": "/usr/bin:/bin", "LANG": "C.UTF-8"},
                                    capture_output=True, timeout=20)
            if result.returncode:
                raise ValueError("official_schema_rejected_" + kind)
    print("Pinned official schemas accepted the catalog, policy and all 18 native EvaluationLogs.")


if __name__ == "__main__":
    try:
        main()
    except Exception as error:
        # Schema diagnostics can echo inputs. Publish only our stable error code.
        code = str(error) if isinstance(error, ValueError) else "schema_validation_failed"
        if not code.replace("_", "").isalnum() or len(code) > 100:
            code = "schema_validation_failed"
        raise SystemExit(code)
