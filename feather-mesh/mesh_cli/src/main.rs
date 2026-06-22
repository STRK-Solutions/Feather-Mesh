const HELP: &str = "\
Feather Mesh CLI

Usage:
  mesh_cli [--help]

Status:
  CLI command parsing and terminal UX live in mesh_cli.
  Business workflows live in mesh_core::services.

Next implementation work should replace this guidance with concrete commands that
call mesh_core services for publishing, discovering, and retrieving data products.
";

fn main() {
    println!("{HELP}");
}
