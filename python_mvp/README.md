# Data Registry CLI (MVP)

Simple in-memory data registry CLI for a data feam capstone project.

The CLI is exposed as a `feam` command with a few MVP subcommands for creating,
searching, and inspecting mock data products.

## Installation (Windows / PowerShell)

1. Make sure you have Python 3.9+ installed and on your `PATH`.
2. Open PowerShell in the project root (the folder that contains `pyproject.toml`).
3. Create and activate a virtual environment:

```powershell
python -m venv .venv
.venv\Scripts\Activate.ps1
```

## Installation (macOS / Unix / Linux)

1. Make sure you have Python 3.9+ installed:
2. Open a terminal in the project root (the folder that contains pyproject.toml).
3. Create and activate a virtual environment:

```bash
python3 -m venv .venv
source .venv/bin/activate
```

4. Install the package in editable mode so the `feam` command is created:

```powershell
pip install -e .
```

After this, `feam` should be available in the activated PowerShell session.

> If PowerShell blocks running the activation script, you can temporarily
> relax the execution policy for the current session:
>
> ```powershell
> Set-ExecutionPolicy -Scope Process -ExecutionPolicy Bypass
> .venv\Scripts\Activate.ps1
> ```

If you make changes and need the `feam` commands to reflect these changes, first uninstall:

```powershell
pip uninstall data-registry-cli
```

Then run:

```powershell
pip install -e .
```

## CLI commands

Once installed, the main entry point is:

```powershell
feam <command> [options]
```

Available commands:

- `feam serve <path> --name <name> --asset-type <type>` – current MVP command name for creating a product entry.
- `feam search <query> [--filter key=value]` – search data products by name containing `<query>`.
- `feam show <product_id> [--version <v>]` – show full details for a single data product by ID.

Current MVP helpers that still exist:

- `feam teams` – list all teams in the mock registry.
- `feam products` – list all data products.

## Example usage (PowerShell)

```powershell
# Create a product entry
feam serve ./path/to/asset --name climate_daily --asset-type dataset

# Search for products whose name contains "climate"
feam search climate --filter status=active

# Show details for product with ID 1
feam show 1 --version 1.0.0
```
