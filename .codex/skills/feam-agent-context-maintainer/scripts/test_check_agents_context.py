#!/usr/bin/env python3
"""Exercise the checker in isolated copies, including its stated semantic limits."""

import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[4]
SCRIPT = Path(".codex/skills/feam-agent-context-maintainer/scripts/check_agents_context.sh")


class ContextCheckerTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="feam context tests ")
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name) / "repo with spaces"
        self.root.mkdir()
        for name in (".codex", ".github", "feather-mesh"):
            shutil.copytree(ROOT / name, self.root / name,
                            ignore=shutil.ignore_patterns("target", "__pycache__", ".DS_Store"))
        for name in ("AGENTS.md", "data_access.md", "data_access_implementation_workplan.md",
                     "data_access_agent_context_review.md", "map.md"):
            shutil.copy2(ROOT / name, self.root / name)
        (self.root / ".agents").mkdir()
        (self.root / ".agents/skills").symlink_to("../.codex/skills", target_is_directory=True)
        self.agents = self.root / "AGENTS.md"
        self.env = {**os.environ, "PATH": f"{Path(sys.executable).parent}{os.pathsep}{os.environ['PATH']}"}

    def run_check(self, *, cwd=None, arguments=(), expected=0):
        result = subprocess.run(["bash", str(self.root / SCRIPT), *map(str, arguments)],
                                cwd=cwd or self.root, env=self.env, text=True,
                                capture_output=True, timeout=15)
        self.assertEqual(result.returncode, expected, result.stdout + result.stderr)
        if expected == 0:
            self.assertIn("Structural checks only", result.stdout)
        return result

    def append(self, text):
        with self.agents.open("a") as output:
            output.write("\n" + text + "\n")

    def test_default_root_independent_of_working_directory(self):
        for cwd in (self.root, self.root / "feather-mesh", Path(self.temp.name)):
            with self.subTest(cwd=cwd):
                result = self.run_check(cwd=cwd)
                self.assertIn(str(self.root.resolve()), result.stdout)

    def test_explicit_root_and_missing_file(self):
        # Call the real checker to prove override selection, not its own default.
        command = ["bash", str(ROOT / SCRIPT), "--root", str(self.root)]
        result = subprocess.run(command, env=self.env, capture_output=True, text=True, timeout=15)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.agents.unlink()
        result = subprocess.run(command, env=self.env, capture_output=True, text=True, timeout=15)
        self.assertEqual(result.returncode, 1)
        self.assertIn("AGENTS.md", result.stderr)

    def test_unknown_argument_is_rejected(self):
        self.run_check(arguments=("--rot", self.root), expected=2)

    def test_missing_requirement_target(self):
        (self.root / "data_access.md").unlink()
        self.assertIn("broken local link", self.run_check(expected=1).stderr)

    def test_requirement_filename_without_routing_link_is_insufficient(self):
        self.agents.write_text(self.agents.read_text().replace(
            "[requirements](data_access.md)", "`data_access.md`"))
        self.assertIn("missing routing link", self.run_check(expected=1).stderr)

    def test_missing_skill_and_broken_resource(self):
        skill = self.root / ".codex/skills/feam-peer-data-access/SKILL.md"
        original = skill.read_text()
        skill.unlink()
        self.run_check(expected=1)
        skill.write_text(original + "\n[resource](missing.md)\n")
        self.assertIn("broken local link", self.run_check(expected=1).stderr)

    def test_invalid_skill_metadata(self):
        skill = self.root / ".codex/skills/feam-peer-data-access/SKILL.md"
        original = skill.read_text()
        for replacement in ("name: wrong-name", "name: [", "name: 42"):
            with self.subTest(replacement=replacement):
                skill.write_text(original.replace("name: feam-peer-data-access", replacement))
                self.run_check(expected=1)

    def test_missing_discovery_link(self):
        (self.root / ".agents/skills").unlink()
        self.assertIn("discovery link", self.run_check(expected=1).stderr)

    def test_missing_validation_command_or_surface(self):
        original = self.agents.read_text()
        for command in ("cargo fmt -- --check", "cargo clippy -- -D warnings", "cargo test"):
            with self.subTest(command=command):
                self.agents.write_text(original.replace(f"`{command}`", "omitted"))
                self.assertIn("validation command", self.run_check(expected=1).stderr)
        for key in ("SDK", "STAC", "Inspectors", "HPC"):
            with self.subTest(key=key):
                self.agents.write_text("\n".join(line for line in original.splitlines()
                                                if not line.startswith(f"| {key} |")))
                self.assertIn("guidance row", self.run_check(expected=1).stderr)

    def test_workspace_and_command_drift(self):
        cargo = self.root / "feather-mesh/Cargo.toml"
        original = cargo.read_text()
        cargo.write_text(original.replace('"mesh_cli",', ""))
        self.assertIn("workspace missing member", self.run_check(expected=1).stderr)
        cargo.write_text(original)
        self.agents.write_text(self.agents.read_text().replace("- `serve`", "- `stac`"))
        self.assertIn("command list", self.run_check(expected=1).stderr)

    def test_review_mutation_removed_layering_and_exit_routing(self):
        original = self.agents.read_text()
        self.agents.write_text("\n".join(line for line in original.splitlines()
                                        if not line.startswith("| mesh_")))
        self.assertIn("guidance row", self.run_check(expected=1).stderr)
        self.agents.write_text("\n".join(line for line in original.splitlines()
                                        if not line.startswith("Stable exit codes")))
        self.assertIn("routing link", self.run_check(expected=1).stderr)

    def test_review_prose_mutations_require_separate_semantic_review(self):
        original = self.agents.read_text()
        mutations = (
            "`python_mvp/` is the primary source of truth.",
            "For peer access SQLite is authoritative; recursively discover all Parquet files.",
            "The earlier Python prototype is historical and is not the current implementation.",
            "Never choose the source of truth from python_mvp/.",
        )
        for mutation in mutations:
            with self.subTest(mutation=mutation):
                self.agents.write_text(original)
                self.append(mutation)
                self.run_check()

    def test_equivalent_prose_is_not_an_exact_wording_contract(self):
        self.agents.write_text(self.agents.read_text().replace(
            "The current implementation lives in", "Active implementation resides in"))
        self.run_check()


if __name__ == "__main__":
    unittest.main(verbosity=2)
