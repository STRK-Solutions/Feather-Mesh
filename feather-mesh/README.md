<div style="text-align: center;">
    <img src="../imgs/logo.png" width="400">
</div>

<!-- # Feather Mesh -->

Feather Mesh (feam) is an HPC-native middleware layer that standardizes how teams publish, discover, and consume reusable data products without forcing teams to give up ownership of their data. It is intended to reduce duplicated work, improve cross-team interoperability, and make pipelines more reliable by replacing ad hoc path conventions with a governed product catalog and deterministic retrieval workflows.

---

## Prerequisites

> **_NOTE:_**  Recommended to use some sort of Linux/Unix environment (WSL is a good option if you're running windows).

Make sure you have Rust installed.

### Install Rust

Using `rustup`:

```bash
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh
```

After installation:

```bash
rustc --version
cargo --version
```

If both return versions, you're good to go!

---

## Project Setup

### 1. Clone the Repository

```bash
git clone https://github.com/saifshai/dcat_4_hpc.git
cd dcat_4_hpc/agr-data-mesh
```

### 2. Install Dependencies

Rust handles dependencies automatically via `Cargo.toml`.

To fetch dependencies:

```bash
cargo fetch
```

---

## Build the Project

### Debug Build (default)

```bash
cargo build
```

Output binary will be located in:

```
target/debug/project-name
```

### Release Build (optimized)

```bash
cargo build --release
```

Output binary:

```
target/release/project-name
```

---

## Run the Project

### Run in Debug Mode

```bash
cargo run
```

### Run in Release Mode

```bash
cargo run --release
```

### Run With Arguments

```bash
cargo run -- --input file.txt
```

(Note the `--` before arguments.)

---

## Run Tests

```bash
cargo test
```

---

## Formatting & Linting

Format code:

```bash
cargo fmt
```

Run Clippy:

```bash
cargo clippy
```

---

## Project Structure

```
project-name/
│
├── Cargo.toml        # Project manifest and dependencies
├── Cargo.lock        # Locked dependency versions
└── src/
    └── main.rs       # Application entry point
```

If it's a library project:

```
src/lib.rs
```

---

## Useful Cargo Commands

| Command       | Description                 |
| ------------- | --------------------------- |
| `cargo check` | Type-check without building |
| `cargo build` | Build project               |
| `cargo run`   | Build and run               |
| `cargo test`  | Run tests                   |
| `cargo clean` | Remove build artifacts      |
