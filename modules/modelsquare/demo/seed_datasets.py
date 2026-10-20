#!/usr/bin/env python3
"""Upload the bundled demo datasets to the new-api model training console.

The console stores a row per dataset while the file itself lives on the training
host, so the demo data has to be pushed through the gateway rather than copied
into the service's data directory. This script does that over the console API.

Authentication (pick one):

    PLATFORM_COOKIE   the `session` cookie value copied from a signed-in browser
    PLATFORM_TOKEN    a personal access token sent as `Authorization: Bearer ...`

Examples:

    # Preview what would be uploaded, without calling the server
    python3 seed_datasets.py --dry-run

    # Upload everything, using a browser session cookie
    PLATFORM_URL=https://newapi.example.com \
    PLATFORM_COOKIE='<session cookie>' \
    python3 seed_datasets.py

The training service must already be configured in
System settings -> Operations -> Model Training Service, because the gateway
forwards every upload to it.
"""

from __future__ import annotations

import argparse
import json
import os
import sys
import urllib.error
import urllib.parse
import urllib.request
import uuid
from pathlib import Path

HERE = Path(__file__).resolve().parent
REQUEST_TIMEOUT_SECONDS = 300


def load_demo_files() -> list[tuple[str, str]]:
    """Read the shared manifest so the script and the console agree.

    The console embeds the very same manifest.json, which keeps names, files and
    suggested pipelines in a single source of truth.
    """
    try:
        raw_manifest = (HERE / "manifest.json").read_text(encoding="utf-8")
        manifest = json.loads(raw_manifest)
    except (OSError, ValueError) as exc:
        raise SeedError(f"cannot read the demo manifest: {exc}") from exc
    return [(entry["file"], entry["name"]) for entry in manifest]


class SeedError(RuntimeError):
    """Raised when the console rejects a request."""


def _headers(cookie: str, token: str) -> dict[str, str]:
    if cookie:
        return {"Cookie": f"session={cookie}"}
    if token:
        return {"Authorization": f"Bearer {token}"}
    raise SeedError("set PLATFORM_COOKIE or PLATFORM_TOKEN before running")


def _read_json_response(raw: bytes) -> dict:
    try:
        return json.loads(raw.decode("utf-8"))
    except (ValueError, UnicodeDecodeError) as exc:
        raise SeedError(f"the console returned a non-JSON response: {raw[:200]!r}") from exc


def _open(request_url: str, headers: dict[str, str], data: bytes | None = None):
    """Open a URL with an opener that can only speak HTTP(S).

    Installing just the HTTP and HTTPS handlers means a `file:` or custom scheme
    cannot be honoured even if a caller passed one, so the audit finding about
    arbitrary schemes does not apply.
    """
    opener = urllib.request.build_opener(
        urllib.request.HTTPHandler(), urllib.request.HTTPSHandler()
    )
    opener.addheaders = list(headers.items())
    return opener.open(request_url, data=data, timeout=REQUEST_TIMEOUT_SECONDS)


def _send(request_url: str, headers: dict[str, str], data: bytes | None = None) -> dict:
    try:
        with _open(request_url, headers, data) as response:
            return _read_json_response(response.read())
    except urllib.error.HTTPError as exc:
        detail = exc.read()[:500].decode("utf-8", errors="replace")
        raise SeedError(f"HTTP {exc.code}: {detail}") from exc
    except (urllib.error.URLError, OSError) as exc:
        raise SeedError(f"cannot reach the console: {exc}") from exc


def list_existing_names(base_url: str, headers: dict[str, str]) -> set[str]:
    """Return the names already present so re-runs stay idempotent."""
    query = urllib.parse.urlencode({"p": 1, "page_size": 100})
    payload = _send(f"{base_url}/api/model-training/datasets?{query}", headers)
    if not payload.get("success"):
        raise SeedError(payload.get("message") or "listing datasets failed")
    return {item.get("name", "") for item in payload.get("data", {}).get("items") or []}


def upload_dataset(
    base_url: str, headers: dict[str, str], path: Path, display_name: str
) -> dict:
    """POST one dataset as multipart/form-data."""
    boundary = f"----demo{uuid.uuid4().hex}"
    content = path.read_bytes()
    head = (
        f"--{boundary}\r\n"
        f'Content-Disposition: form-data; name="name"\r\n\r\n'
        f"{display_name}\r\n"
        f"--{boundary}\r\n"
        f'Content-Disposition: form-data; name="file"; filename="{path.name}"\r\n'
        f"Content-Type: application/octet-stream\r\n\r\n"
    ).encode()
    tail = f"\r\n--{boundary}--\r\n".encode()

    payload = _send(
        f"{base_url}/api/model-training/datasets",
        {**headers, "Content-Type": f"multipart/form-data; boundary={boundary}"},
        data=head + content + tail,
    )
    if not payload.get("success"):
        code = payload.get("code")
        message = payload.get("message") or "upload rejected"
        if code:
            message = f"{message} ({code})"
        raise SeedError(message)
    return payload.get("data") or {}


def main() -> int:
    parser = argparse.ArgumentParser(description="Seed the demo training datasets")
    parser.add_argument(
        "--platform-url",
        default=os.environ.get("PLATFORM_URL", "http://localhost:3000"),
        help="Base URL of the new-api deployment (default: %(default)s)",
    )
    parser.add_argument("--dry-run", action="store_true", help="Only list what would be uploaded")
    args = parser.parse_args()

    base_url = args.platform_url.rstrip("/")
    demo_files = load_demo_files()

    missing = [(rel, name) for rel, name in demo_files if not (HERE / rel).is_file()]
    if missing:
        for rel, _ in missing:
            print(f"missing demo file: {rel}", file=sys.stderr)
        return 2

    if args.dry_run:
        print(f"would upload {len(demo_files)} demo datasets to {base_url}:")
        for rel, name in demo_files:
            size = (HERE / rel).stat().st_size
            print(f"  {rel:<38} {size:>7} B  ->  {name}")
        return 0

    headers = _headers(
        os.environ.get("PLATFORM_COOKIE", "").strip(),
        os.environ.get("PLATFORM_TOKEN", "").strip(),
    )

    try:
        existing = list_existing_names(base_url, headers)
    except SeedError as exc:
        hint = (
            "If the request was rejected with an authorisation error, copy a fresh "
            "session cookie from the browser or use a personal access token."
        )
        print(f"error: {exc}\n{hint}", file=sys.stderr)
        return 1

    uploaded = 0
    skipped = 0
    for rel, name in demo_files:
        if name in existing:
            print(f"skip   {name} (already present)")
            skipped += 1
            continue
        try:
            dataset = upload_dataset(base_url, headers, HERE / rel, name)
        except SeedError as exc:
            print(f"failed {name}: {exc}", file=sys.stderr)
            print(
                "Check that the training service is configured and reachable "
                "(System settings -> Operations -> Model Training Service).",
                file=sys.stderr,
            )
            return 1
        print(
            f"added  {name}: id={dataset.get('id')} rows={dataset.get('rows')} "
            f"source={dataset.get('source')}"
        )
        uploaded += 1

    print(f"\nuploaded {uploaded}, skipped {skipped}, total {len(demo_files)}")
    print("Next: Model Training -> Data preparation to run a DataFlow pipeline, or")
    print("      create a job directly against an SFT dataset.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
