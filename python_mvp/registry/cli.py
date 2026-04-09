from __future__ import annotations

import argparse
from typing import List, Optional

from registry.services import Registry
from registry.seed import seed_mock_data

import json
from pathlib import Path

DATA_FILE = Path("feam_registry.json")


def load_registry(registry: Registry) -> None:
	"""Load registry state from disk, with a fallback migration for old files."""
	if not DATA_FILE.exists():
		seed_mock_data(registry)
		return

	with DATA_FILE.open("r", encoding="utf-8") as f:
		data = json.load(f)

	for team in data.get("teams", []):
		registry.create_team(team["name"])

	for product in data.get("products", []):
		registry.create_data_product(
			name=product["name"],
			description=product["description"],
			owner_team_id=product["owner_team_id"],
			data_format=product["data_format"],
			access_uri=product["access_uri"],
			status=product["status"],
			classification=product["classification"],
		)

	metadata_entries = data.get("metadata")
	if metadata_entries is None:
		restore_seed_metadata(registry)
		return

	for entry in metadata_entries:
		registry.add_metadata(
			data_product_id=entry["data_product_id"],
			namespace=entry["namespace"],
			meta_key=entry["meta_key"],
			meta_value=entry["meta_value"],
			value_type=entry["value_type"],
		)


def restore_seed_metadata(registry: Registry) -> None:
	"""Backfill metadata for old JSON files that predate metadata persistence."""
	seeded_registry = Registry()
	seed_mock_data(seeded_registry)

	seeded_products = {product.name: product for product in seeded_registry.list_products()}
	for product in registry.list_products():
		seeded_product = seeded_products.get(product.name)
		if seeded_product is None or product.metadata:
			continue
		for entry in seeded_product.metadata:
			registry.add_metadata(
				data_product_id=product.product_id,
				namespace=entry.namespace,
				meta_key=entry.meta_key,
				meta_value=entry.meta_value,
				value_type=entry.value_type,
			)


def save_registry(registry: Registry) -> None:
	data = {
		"teams": [{"name": t.name} for t in registry.list_teams()],
		"products": [
			{
				"name": p.name,
				"description": p.description,
				"owner_team_id": p.owner_team_id,
				"data_format": p.data_format,
				"access_uri": p.access_uri,
				"status": p.status,
				"classification": p.classification,
			}
			for p in registry.list_products()
		],
		"metadata": [
			{
				"data_product_id": p.product_id,
				"namespace": m.namespace,
				"meta_key": m.meta_key,
				"meta_value": m.meta_value,
				"value_type": m.value_type,
			}
			for p in registry.list_products()
			for m in p.metadata
		],
	}

	with DATA_FILE.open("w", encoding="utf-8") as f:
		json.dump(data, f, indent=2)


def print_header(title: str) -> None:
	print("\n" + "=" * 80)
	print(title.center(80))
	print("=" * 80)


def print_teams(registry: Registry) -> None:
	print_header("Teams")
	for t in registry.list_teams():
		print(f"[{t.teams_id}] {t.name} (created {t.created_at:%Y-%m-%d})")


def print_products(registry: Registry) -> None:
	print_header("Data products")
	for p in registry.list_products():
		team = registry.get_team(p.owner_team_id)
		owner = team.name if team else f"team:{p.owner_team_id}"
		print(f"[{p.product_id}] {p.name} (owner={owner})")
		# print(
		# 	f"[{p.product_id}] {p.name} | owner={owner} | format={p.data_format} | "
		# 	f"status={p.status} | class={p.classification}"
		# )


def print_product_details(registry: Registry, product_id: int) -> None:
	product = registry.get_product(product_id)
	if not product:
		print(f"No data product with id {product_id} found.")
		return

	team = registry.get_team(product.owner_team_id)
	owner = team.name if team else f"team:{product.owner_team_id}"

	print_header(f"Data product: {product.name}")
	print(f"ID          : {product.product_id}")
	print(f"Description : {product.description}")
	print(f"Owner team  : {owner}")
	print(f"Format      : {product.data_format}")
	print(f"Access URI  : {product.access_uri}")
	print(f"Status      : {product.status}")
	print(f"Classif.    : {product.classification}")
	print(f"Created     : {product.created_at:%Y-%m-%d %H:%M}")
	print(f"Updated     : {product.updated_at:%Y-%m-%d %H:%M}")

	if not product.metadata:
		print("\nNo metadata entries.")
		return

	print("\nMetadata:")
	for m in product.metadata:
		print(
			f"- {m.namespace}.{m.meta_key} = {m.meta_value} "
			f"(type={m.value_type})"
		)
  
	print()


def add_product_interactively(registry: Registry) -> None:
	print_header("Add new data product")
	name = input("Name: ").strip()
	description = input("Description: ").strip()
	data_format = input("Data format (e.g. parquet, delta): ").strip() or "parquet"
	access_uri = input("Access URI: ").strip()
	status = input("Status (active, deprecated, draft): ").strip() or "draft"
	classification = (
		input("Classification (internal/restricted/public): ").strip() or "internal"
	)

	print("\nAvailable teams:")
	for team in registry.list_teams():
		print(f"[{team.teams_id}] {team.name}")

	while True:
		raw = input("Owner team id: ").strip()
		try:
			owner_team_id = int(raw)
		except ValueError:
			print("Please enter a numeric team id.")
			continue
		if registry.get_team(owner_team_id) is None:
			print("Unknown team id, try again.")
			continue
		break

	product = registry.create_data_product(
		name=name,
		description=description,
		owner_team_id=owner_team_id,
		data_format=data_format,
		access_uri=access_uri,
		status=status,
		classification=classification,
	)

	print(f"\nCreated data product {product.name} with id {product.product_id}.")


def search_products(registry: Registry) -> None:
	term = input("Search term in product name: ").strip()
	results = registry.search_products_by_name(term)
	if not results:
		print("No products found.")
		return
	print_header(f"Search results for '{term}'")
	for p in results:
		team = registry.get_team(p.owner_team_id)
		owner = team.name if team else f"team:{p.owner_team_id}"
		print(f"[{p.product_id}] {p.name} (owner={owner})")


def build_parser() -> argparse.ArgumentParser:
	"""Create the top-level argument parser for the `feam` executable."""

	parser = argparse.ArgumentParser(
		prog="feam",
		description="Data registry CLI (mock MVP)",
	)
	subparsers = parser.add_subparsers(dest="command", required=True)

	# Legacy MVP helper commands
	subparsers.add_parser("teams", help="List teams")

	# Legacy MVP helper commands
	subparsers.add_parser("products", help="List data products")

	# feam show <product_id> [--version <v>]
	show_parser = subparsers.add_parser(
		"show",
		help="Show a data product by id",
	)
	show_parser.add_argument("product_id", type=int, help="Data product id")
	show_parser.add_argument(
		"--version",
		help="Optional product version to inspect",
	)

	# feam search <query> [--filter key=value]
	search_parser = subparsers.add_parser(
		"search",
		help="Search data products by name",
	)
	search_parser.add_argument("query", help="Search term in product name")
	search_parser.add_argument(
		"--filter",
		dest="filter_expr",
		help="Optional filter expression such as key=value",
	)

	# feam serve <path> --name <name> --asset-type <type> [flags]
	serve_parser = subparsers.add_parser(
		"serve",
		help="Create a new data product entry",
	)
	serve_parser.add_argument("path", nargs="?", help="Asset path to serve")
	serve_parser.add_argument("--name", help="Asset name")
	serve_parser.add_argument("--asset-type", help="Asset type")

	return parser





def run(argv: Optional[List[str]] = None) -> None:
	parser = build_parser()
	args = parser.parse_args(argv)

	registry = Registry()
	load_registry(registry)

	cmd = args.command

	if cmd == "teams":
		print_teams(registry)
	elif cmd == "products":
		print_products(registry)
	elif cmd == "show":
		print_product_details(registry, args.product_id)
	elif cmd == "search":
		results = registry.search_products_by_name(args.query)
		if not results:
			print(f"No products found for term '{args.query}'.")
		else:
			print_header(f"Search results for '{args.query}'")
			for p in results:
				team = registry.get_team(p.owner_team_id)
				owner = team.name if team else f"team:{p.owner_team_id}"
				print(f"[{p.product_id}] {p.name} (owner={owner})")
	elif cmd == "serve":
		add_product_interactively(registry)
	else:
		parser.error(f"Unknown feam subcommand: {cmd}")

	save_registry(registry)



def main() -> None:
	"""Compatibility wrapper for `python cli.py` / `python main.py`.

	This just forwards to `run()` which implements the Unix-style CLI.
	"""
	run()


if __name__ == "__main__":
	main()
