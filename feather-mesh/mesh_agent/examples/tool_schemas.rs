//! Export the current provider-neutral tools for reproducible router screening.
fn main() -> Result<(), serde_json::Error> {
    println!(
        "{}",
        serde_json::to_string_pretty(&mesh_agent::tools::tool_schemas())?
    );
    Ok(())
}
