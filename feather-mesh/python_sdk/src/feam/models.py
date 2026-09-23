"""Descriptor objects returned by the Rust resolver."""

from dataclasses import dataclass
from pathlib import Path
from typing import Any


@dataclass(frozen=True)
class Asset:
    id: str
    path: Path
    media_type: str
    role: str
    size: int
    sha256: str | None


@dataclass(frozen=True)
class Product:
    namespace: str
    product_id: str
    version: str
    manifest_revision: int
    data_kind: str
    data_format: str
    assets: tuple[Asset, ...]
    metadata: dict[str, Any]

    @property
    def reference(self) -> str:
        return f"product://{self.namespace}/{self.product_id}"

    @property
    def paths(self) -> list[Path]:
        return [asset.path for asset in self.assets]

    def asset(self, asset_id: str) -> Asset:
        for asset in self.assets:
            if asset.id == asset_id:
                return asset
        raise KeyError(f"registered asset {asset_id!r} is absent from {self.reference}@{self.version}")


def product_from_wire(value: dict[str, Any]) -> Product:
    assets = tuple(
        Asset(
            id=asset["id"],
            path=Path(asset["project_access_path"]),
            media_type=asset["media_type"],
            role=asset["role"],
            size=asset["size"],
            sha256=asset.get("sha256"),
        )
        for asset in value["assets"]
    )
    return Product(
        namespace=value["namespace"],
        product_id=value["product_id"],
        version=value["version"],
        manifest_revision=value["manifest_revision"],
        data_kind=value["data_kind"],
        data_format=value["data_format"],
        assets=assets,
        metadata=value,
    )
