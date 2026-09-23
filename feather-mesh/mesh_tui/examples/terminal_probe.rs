fn main() {
    let panic = std::env::args().nth(1).as_deref() == Some("panic");
    if let Err(error) = mesh_tui::terminal_lifecycle_probe(panic) {
        eprintln!("{error}");
        std::process::exit(1);
    }
}
