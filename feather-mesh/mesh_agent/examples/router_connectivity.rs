//! Public, unauthenticated TLS diagnostic; no inference request or credential.
fn main() {
    let runtime = tokio::runtime::Builder::new_current_thread()
        .enable_all()
        .build()
        .unwrap();
    runtime.block_on(async {
        match reqwest::Client::builder()
            .use_rustls_tls()
            .build()
            .unwrap()
            .get("https://openrouter.ai/api/v1/models/deepseek/deepseek-v4.1-flash/endpoints")
            .send()
            .await
        {
            Ok(response) => println!("Public model metadata HTTP {}", response.status()),
            Err(error) => {
                eprintln!("{error:?}");
                std::process::exit(1);
            }
        }
    });
}
