import json
import stat
import sys
from pathlib import Path

import pytest

from feam import Project
from feam.exceptions import ExecutableNotFoundError, NotFoundError, ProtocolError


def executable(tmp_path: Path, stdout: str, stderr: str = "", code: int = 0) -> Path:
    script = tmp_path / "fake-feam.py"
    script.write_text(
        "#!" + sys.executable + "\n"
        + "import sys\n"
        + f"sys.stdout.write({stdout!r})\n"
        + f"sys.stderr.write({stderr!r})\n"
        + f"raise SystemExit({code})\n",
        encoding="utf-8",
    )
    script.chmod(script.stat().st_mode | stat.S_IXUSR)
    return script


def test_resolve_uses_an_argument_array_and_parses_the_pinned_descriptor(tmp_path: Path):
    wire = {
        "protocol": "feam.peer.v1",
        "namespace": "climate",
        "product_id": "observations",
        "version": "v1",
        "manifest_revision": 1,
        "data_kind": "table",
        "data_format": "parquet",
        "assets": [
            {
                "id": "data",
                "project_access_path": "/tmp/path with spaces/data.parquet",
                "media_type": "application/vnd.apache.parquet",
                "role": "data",
                "size": 10,
                "sha256": None,
            }
        ],
    }
    root = tmp_path / "client project"
    root.mkdir()
    project = Project.open(root, executable=executable(tmp_path, json.dumps(wire)))
    table = project.resolve_table("product://climate/observations", version="v1")
    assert table.paths == [Path("/tmp/path with spaces/data.parquet")]


def test_missing_executable_malformed_json_and_typed_error_are_distinct(tmp_path: Path):
    root = tmp_path / "project"
    root.mkdir()
    with pytest.raises(ExecutableNotFoundError):
        Project.open(root, executable=tmp_path / "missing").resolve("product://climate/x", version="v1")
    with pytest.raises(ProtocolError):
        Project.open(root, executable=executable(tmp_path, "not-json")).resolve("product://climate/x", version="v1")
    error = {"protocol": "feam.peer.v1", "error": {"kind": "dataset_not_registered", "message": "not registered"}}
    with pytest.raises(NotFoundError):
        Project.open(root, executable=executable(tmp_path, "", json.dumps(error), 4)).resolve("product://climate/x", version="v1")
