//! Offline, exact-loopback compatibility probe for the Go HTTPS/socket broker.
//! It cannot select an external endpoint or invoke FEAM mutations.
use mesh_agent::AgentConfig;
use mesh_agent::provider::{
    CancellationToken, ModelMessage, ModelProvider, ModelRequest, ProviderEvent, RouterProvider,
};

fn main() -> Result<(), Box<dyn std::error::Error>> {
    let path = std::env::args().nth(1).ok_or("expected fixture profile")?;
    let config = AgentConfig::load(path)?;
    let (_, profile) = config.selected(None)?;
    if profile.loopback_ca_file.is_none() {
        return Err("probe requires scoped HTTPS-loopback trust".into());
    }
    profile.validate("probe")?;
    let mut provider = RouterProvider::from_profile(profile.clone())?;
    provider.set_request_id(Some("00000000-0000-4000-8000-000000000002".into()));
    let mut text = String::new();
    let mut usage = false;
    let mut finished = false;
    let mut complete_tool = false;
    provider.stream(
        ModelRequest {
            messages: vec![ModelMessage {
                role: "user".into(),
                content: serde_json::json!("Synthetic offline broker check."),
                tool_call_id: None,
                tool_calls: None,
            }],
            tools: mesh_agent::tools::tool_schemas(),
            max_response_chars: 8000,
        },
        &CancellationToken::default(),
        &mut |event| match event {
            ProviderEvent::TextDelta(delta) => text.push_str(&delta),
            ProviderEvent::Usage(_) => usage = true,
            ProviderEvent::Finished => finished = true,
            ProviderEvent::ToolCall(call) => {
                complete_tool = call.id == "fake-call"
                    && call.name == "help.lookup"
                    && call.arguments == serde_json::json!({"topic":"resolve"});
            }
        },
    )?;
    if !finished || !usage || !complete_tool || text != "Synthetic broker response." {
        return Err("incomplete synthetic exchange".into());
    }
    println!(
        "PASS: Rust router -> HTTPS loopback -> private Unix socket -> fake broker; usage complete"
    );
    Ok(())
}
