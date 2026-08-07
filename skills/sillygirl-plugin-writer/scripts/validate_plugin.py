#!/usr/bin/env python3
"""Validate SillyGirl JavaScript and Python plugin source files."""

from __future__ import annotations

import argparse
import ast
import json
import re
import shutil
import subprocess
import sys
from pathlib import Path

META_RE = re.compile(
    r"^\s*(?://|#+)\s*\[\s*([\w+-]+)\s*:\s*(.*?)\s*\]\s*$", re.MULTILINE
)
REQUIRED = ("title", "name", "description", "version")
ACTIVATORS = ("rule", "cron", "on_start", "web")
FORBIDDEN = ("[param:", "BncrDB", "BncrCreateSchema", "BncrPluginConfig", "SillyGirlPluginConfig")
SAFE_NAME_RE = re.compile(r"^[A-Za-z0-9._-]+$")
VERSION_RE = re.compile(r"^v?\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.-]+)?$")


def metadata(source: str) -> dict[str, list[str]]:
    result: dict[str, list[str]] = {}
    for key, value in META_RE.findall(source):
        result.setdefault(key.lower(), []).append(value.strip())
    if "desc" in result and "description" not in result:
        result["description"] = result["desc"]
    return result


def python_unawaited_sender_calls(tree: ast.AST) -> list[int]:
    parents: dict[ast.AST, ast.AST] = {}
    for parent in ast.walk(tree):
        for child in ast.iter_child_nodes(parent):
            parents[child] = parent

    lines: list[int] = []
    for node in ast.walk(tree):
        if not isinstance(node, ast.Call) or not isinstance(node.func, ast.Attribute):
            continue
        owner = node.func.value
        if not isinstance(owner, ast.Name) or owner.id != "s":
            continue
        cursor: ast.AST | None = node
        awaited = False
        while cursor in parents:
            cursor = parents[cursor]
            if isinstance(cursor, ast.Await):
                awaited = True
                break
            if isinstance(cursor, (ast.stmt, ast.Lambda)):
                break
        if not awaited:
            lines.append(node.lineno)
    return sorted(set(lines))


def validate(path: Path, allow_inactive: bool) -> dict[str, object]:
    errors: list[str] = []
    warnings: list[str] = []
    try:
        source = path.read_text(encoding="utf-8")
    except (OSError, UnicodeError) as exc:
        return {"file": str(path), "valid": False, "errors": [str(exc)], "warnings": []}

    meta = metadata(source)
    for key in REQUIRED:
        if not meta.get(key) or not meta[key][0]:
            errors.append(f"missing metadata: {key}")
    if not allow_inactive and not any(meta.get(key) for key in ACTIVATORS):
        errors.append("missing activation metadata: rule, cron, on_start, or web")

    if meta.get("name") and not SAFE_NAME_RE.fullmatch(meta["name"][0]):
        errors.append("metadata name must match [A-Za-z0-9._-]+")
    if meta.get("version") and not VERSION_RE.fullmatch(meta["version"][0]):
        warnings.append("version is not semantic version format (v1.2.3)")
    for value in meta.get("depe", []):
        try:
            parsed = json.loads(value)
            if not isinstance(parsed, list) or not all(isinstance(item, str) for item in parsed):
                raise ValueError("not a string array")
        except (json.JSONDecodeError, ValueError) as exc:
            errors.append(f"depe must be a JSON string array: {exc}")

    for token in FORBIDDEN:
        if token in source:
            errors.append(f"obsolete API or metadata: {token}")
    if re.search(r"(?m)^\s*(?://|#)\s*@(?:title|name|description|desc|version|rule|cron)\b", source):
        warnings.append("legacy @ metadata detected; use [key: value] for new plugins")
    if re.search(r"\bTODO\b", source, re.IGNORECASE):
        warnings.append("TODO placeholder remains")

    suffix = path.suffix.lower()
    if suffix in (".js", ".cjs"):
        if 'require("sillygirl")' not in source and "require('sillygirl')" not in source:
            warnings.append("JavaScript plugin does not import sillygirl")
        node = shutil.which("node")
        if node:
            proc = subprocess.run([node, "--check", str(path)], capture_output=True, text=True)
            if proc.returncode:
                errors.append("node --check: " + (proc.stderr.strip() or proc.stdout.strip()))
        else:
            warnings.append("node not found; syntax check skipped")
    elif suffix == ".py":
        try:
            tree = ast.parse(source, filename=str(path))
        except SyntaxError as exc:
            errors.append(f"Python syntax: line {exc.lineno}: {exc.msg}")
        else:
            if "sillygirl" not in source:
                warnings.append("Python plugin does not import sillygirl")
            lines = python_unawaited_sender_calls(tree)
            if lines:
                errors.append("sender calls must be awaited at lines: " + ", ".join(map(str, lines)))
            has_async_main = any(isinstance(node, ast.AsyncFunctionDef) and node.name == "main" for node in ast.walk(tree))
            if has_async_main and "asyncio.run(main())" not in source:
                warnings.append("async main is not started with asyncio.run(main())")
    else:
        errors.append("unsupported extension; expected .js, .cjs, or .py")

    return {"file": str(path), "valid": not errors, "errors": errors, "warnings": warnings}


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("files", nargs="+", type=Path)
    parser.add_argument("--allow-inactive", action="store_true")
    parser.add_argument("--json", action="store_true", dest="as_json")
    args = parser.parse_args()
    results = [validate(path, args.allow_inactive) for path in args.files]

    if args.as_json:
        print(json.dumps(results, ensure_ascii=False, indent=2))
    else:
        for result in results:
            print(f"{'PASS' if result['valid'] else 'FAIL'} {result['file']}")
            for warning in result["warnings"]:
                print(f"  WARN: {warning}")
            for error in result["errors"]:
                print(f"  ERROR: {error}")
    return 0 if all(result["valid"] for result in results) else 1


if __name__ == "__main__":
    sys.exit(main())
