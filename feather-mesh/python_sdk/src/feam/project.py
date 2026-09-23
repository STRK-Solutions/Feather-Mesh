"""Thin subprocess adapter; all visibility and lifecycle checks remain in Rust."""

from __future__ import annotations

import json
import os
import subprocess
from pathlib import Path
from typing import Any

from .exceptions import ExecutableNotFoundError, ProtocolError, error_for
from .models import Asset, Product, product_from_wire

PROTOCOL = "feam.peer.v1"


class Project:
    def __init__(self, root: Path, executable: str | os.PathLike[str] | None = None):
        self.root = root.resolve()
        self.executable = os.fspath(executable or os.environ.get("FEAM_EXECUTABLE", "feam"))

    @classmethod
    def open(cls, root: str | os.PathLike[str], *, executable: str | os.PathLike[str] | None = None) -> "Project":
        root_path = Path(root)
        if not root_path.is_dir():
            raise ValueError(f"project root does not exist: {root_path}")
        return cls(root_path, executable)

    def resolve(self, reference: str, *, version: str, verify_integrity: bool = False) -> Product:
        if not version:
            raise ValueError("a pinned version is required for direct data access")
        args = ["resolve", reference, "--version", version]
        if verify_integrity:
            args.append("--verify-integrity")
        return product_from_wire(self._call(args))

    def resolve_asset(self, reference: str, *, version: str, asset: str) -> Asset:
        if not version:
            raise ValueError("a pinned version is required for direct data access")
        value = self._call(["resolve", reference, "--version", version, "--asset", asset])
        product = product_from_wire(value)
        if len(product.assets) != 1:
            raise ProtocolError("resolve --asset did not return exactly one asset", kind="protocol_error")
        return product.assets[0]

    def resolve_table(self, reference: str, *, version: str) -> Product:
        product = self.resolve(reference, version=version)
        if product.data_kind != "table" or product.data_format != "parquet":
            raise ValueError(f"{product.reference}@{product.version} is not a registered Parquet table")
        return product

    def scan_table(self, reference: str, *, version: str):
        """Return a native lazy Polars scan over the manifest's exact file list.

        The resolver runs before this call.  Polars opens files lazily, so peer
        link/permission/byte changes after construction can correctly fail at
        ``collect()``; the SDK cannot revalidate a native LazyFrame per read.
        """
        table = self.resolve_table(reference, version=version)
        try:
            import polars as pl
        except ImportError as error:
            raise ImportError("install feam[table] to use scan_table") from error
        return pl.scan_parquet([str(path) for path in table.paths], glob=False, hive_partitioning=False)

    def _call(self, arguments: list[str]) -> dict[str, Any]:
        command = [self.executable, "--project", os.fspath(self.root), "--format", "json", *arguments]
        try:
            completed = subprocess.run(command, check=False, capture_output=True, text=True)
        except FileNotFoundError as error:
            raise ExecutableNotFoundError(
                f"feam executable was not found: {self.executable}", kind="executable_not_found"
            ) from error
        if completed.returncode:
            raise self._error_from_stderr(completed.stderr, completed.returncode)
        try:
            value = json.loads(completed.stdout)
        except json.JSONDecodeError as error:
            raise ProtocolError("feam returned malformed JSON on stdout", kind="malformed_json") from error
        if not isinstance(value, dict) or value.get("protocol") != PROTOCOL:
            raise ProtocolError("feam returned an incompatible peer protocol", kind="incompatible_protocol")
        return value

    @staticmethod
    def _error_from_stderr(stderr: str, returncode: int):
        try:
            value = json.loads(stderr)
            error = value["error"]
            if value.get("protocol") == PROTOCOL and isinstance(error.get("kind"), str):
                return error_for(error["kind"], error.get("message", "feam failed"), returncode)
        except (json.JSONDecodeError, KeyError, TypeError):
            pass
        return ProtocolError(
            f"feam failed with exit code {returncode} without a compatible structured error: {stderr.strip()}",
            kind="unstructured_subprocess_error",
            returncode=returncode,
        )
