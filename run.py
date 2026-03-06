#!/usr/bin/env python3
"""Start NoteDiscovery Go backend."""

import os
import subprocess
import sys
from pathlib import Path


def get_port() -> str:
    if os.getenv("PORT"):
        return os.getenv("PORT")
    return "8000"


def main() -> int:
    Path("data").mkdir(parents=True, exist_ok=True)
    Path("plugins").mkdir(parents=True, exist_ok=True)

    port = get_port()
    print(f"Starting NoteDiscovery Go backend on http://localhost:{port}")

    cmd = [
        "go",
        "run",
        "./cmd/notediscovery",
        "-config",
        "config.yaml",
        "-port",
        port,
    ]
    return subprocess.call(cmd)


if __name__ == "__main__":
    sys.exit(main())
