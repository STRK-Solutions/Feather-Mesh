#!/usr/bin/env python3
"""Check documented structure, not prose semantics or implementation readiness."""

import argparse
from pathlib import Path
import re
import sys
from urllib.parse import unquote, urlsplit

if sys.version_info < (3, 11):
    sys.exit("context checker requires Python 3.11+")

import tomllib

try:
    import yaml
except ImportError:
    sys.exit("missing PyYAML; install scripts/requirements.txt in a virtual environment")


SKILLS = (
    "feam-agent-context-maintainer",
    "feam-rust-workflow",
    "feam-cli-contract",
    "feam-peer-data-access",
)
ROUTES = (
    "data_access.md",
    "data_access_implementation_workplan.md",
    "feather-mesh/.agents/feather_mesh_workplan.md",
    "feather-mesh/mesh_core/Cargo.toml",
    "feather-mesh/mesh_cli/Cargo.toml",
    "feather-mesh/README.md",
    "feather-mesh/mesh_cli/tests/cli_workflow_tests.rs",
    *(f".codex/skills/{name}/SKILL.md" for name in SKILLS),
)
OWNERS = {
    "mesh_cli": "feather-mesh/mesh_cli/src/main.rs",
    "mesh_core::services": "feather-mesh/mesh_core/src/services",
    "mesh_core::repositories": "feather-mesh/mesh_core/src/repositories",
    "mesh_core shared types": "feather-mesh/mesh_core/src/domain.rs",
}
RUST_CHECKS = ("cargo fmt -- --check", "cargo clippy -- -D warnings", "cargo test")


class ContextError(Exception):
    pass


def require(condition, message):
    if not condition:
        raise ContextError(message)


def read(path):
    require(path.is_file(), f"missing file: {path}")
    return path.read_text(encoding="utf-8")


def prose(text):
    """Ignore examples in fenced blocks when inspecting links and table rows."""
    return re.sub(r"^```[^\n]*\n.*?^```\s*$", "", text, flags=re.M | re.S)


def heading_ids(text):
    # Only simple GitHub-style headings used by this repository are supported.
    return {
        re.sub(r"[^\w\- ]", "", title.lower()).replace(" ", "-")
        for title in re.findall(r"^#{1,6}\s+(.+)$", text, re.M)
    }


def check_links(root, document, text):
    """Check local inline Markdown links; external URLs are not fetched."""
    targets = set()
    for raw in re.findall(r"\[[^\]\n]+\]\(([^)\n]+)\)", prose(text)):
        url = urlsplit(raw.strip("<>"))
        if url.scheme or url.netloc:
            continue
        link_path = unquote(url.path)
        if link_path.startswith("/"):
            target = root / link_path.lstrip("/")
        else:
            target = document.parent / link_path if link_path else document
        target = target.resolve()
        require(target.exists(), f"broken local link in {document}: {raw}")
        targets.add(target)
        if url.fragment and target.suffix == ".md":
            require(
                unquote(url.fragment) in heading_ids(read(target)),
                f"missing heading in {document}: {raw}",
            )
    return targets


def check_skill(path):
    text = read(path)
    parts = text.split("---\n", 2)
    require(len(parts) == 3 and parts[0] == "", f"invalid skill frontmatter: {path}")
    try:
        metadata = yaml.safe_load(parts[1])
    except yaml.YAMLError as error:
        raise ContextError(f"invalid skill YAML: {path}: {error}") from error
    require(isinstance(metadata, dict), f"skill metadata must be a mapping: {path}")
    require(metadata.get("name") == path.parent.name, f"skill name/path mismatch: {path}")
    description = metadata.get("description")
    require(
        isinstance(description, str) and 0 < len(description.strip()) <= 1024,
        f"missing/invalid skill description: {path}",
    )
    require(parts[2].strip(), f"empty skill instructions: {path}")
    require("[TODO:" not in text, f"unfinished skill scaffold: {path}")
    return text


def check(root):
    agents_path = root / "AGENTS.md"
    agents = read(agents_path)
    links = check_links(root, agents_path, agents)
    for route in ROUTES:
        require((root / route).resolve() in links, f"AGENTS.md missing routing link: {route}")

    for name in SKILLS:
        path = root / ".codex/skills" / name / "SKILL.md"
        check_links(root, path, check_skill(path))
        discovered_path = root / ".agents/skills" / name / "SKILL.md"
        require(
            discovered_path.resolve() == path.resolve() and discovered_path.is_file(),
            f"missing discovery link to canonical skill: {discovered_path}",
        )

    for route in ROUTES[:3]:
        path = root / route
        check_links(root, path, read(path))

    rows = {}
    for line in prose(agents).splitlines():
        if line.startswith("|"):
            cells = [cell.strip() for cell in line.strip("|").split("|")]
            if len(cells) == 2:
                rows[cells[0]] = cells[1]
    for key in (*OWNERS, "SDK", "STAC", "Inspectors", "HPC"):
        require(rows.get(key), f"AGENTS.md missing nonempty guidance row: {key}")
    for owner, path in OWNERS.items():
        require((root / path).exists(), f"missing {owner} source anchor: {path}")
    code_spans = set(re.findall(r"`([^`\n]+)`", prose(agents)))
    for command in RUST_CHECKS:
        require(command in code_spans, f"AGENTS.md missing validation command: {command}")

    workspace = root / "feather-mesh"
    cargo = tomllib.loads(read(workspace / "Cargo.toml"))
    members = cargo.get("workspace", {}).get("members", [])
    for name in ("mesh_core", "mesh_cli"):
        require(name in members, f"Cargo workspace missing member: {name}")
        manifest = tomllib.loads(read(workspace / name / "Cargo.toml"))
        require(manifest.get("package", {}).get("name") == name, f"wrong package name: {name}")

    source = read(workspace / "mesh_cli/src/main.rs")
    require(re.search(r'#\[command\(name\s*=\s*"feam"', source), "missing feam CLI name anchor")
    command_enum = re.search(r"^enum Command \{\n(.*?)^\}", source, re.M | re.S)
    require(command_enum, "missing CLI Command enum; update structural checker if parsing moved")
    variants = re.findall(r"^    (\w+)(?:,| \{)$", command_enum[1], re.M)
    commands = {re.sub(r"(?<!^)([A-Z])", r"-\1", variant).lower() for variant in variants}
    documented = set(re.findall(r"^- `([a-z][a-z-]*)`$", prose(agents), re.M))
    require(commands and commands == documented, "AGENTS.md command list differs from CLI Command anchors")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", type=Path, default=Path(__file__).resolve().parents[4],
                        help="repository root; default is relative to this script, never cwd")
    args = parser.parse_args()
    root = args.root.resolve()
    try:
        check(root)
    except (ContextError, OSError, ValueError) as error:
        print(f"context structural check failed: {error}", file=sys.stderr)
        return 1
    print(f"Context structural checks passed for {root}: links, skills, workspace/CLI anchors, guidance rows.")
    print("Structural checks only; semantic review and affected-surface runtime/HPC evidence remain required.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
