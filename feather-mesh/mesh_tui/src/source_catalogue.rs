use std::collections::{HashMap, HashSet};
use std::fs;
use std::io::Read;
use std::path::{Component, Path, PathBuf};

use serde::Deserialize;

const MAX_FILE_BYTES: u64 = 1024 * 1024;
const MAX_SUITES: usize = 32;
const MAX_CATEGORIES_PER_SUITE: usize = 128;
const MAX_PRODUCTS_PER_CATEGORY: usize = 2048;
const MAX_VARIANTS_PER_PRODUCT: usize = 128;

#[derive(Debug, Clone)]
pub struct SourceCatalogueEntry {
    pub label: String,
    pub reference: String,
    pub detail: String,
}

#[derive(Debug, Clone)]
pub struct SourceCatalogue {
    pub path: PathBuf,
    pub title: String,
    pub entries: Vec<SourceCatalogueEntry>,
    pub suite_count: usize,
    pub product_count: usize,
}

impl SourceCatalogue {
    pub fn selected_detail(&self, selected: usize) -> Option<&str> {
        self.entries
            .get(selected)
            .map(|entry| entry.detail.as_str())
    }

    pub fn export_text(&self) -> String {
        let mut text = format!(
            "Source catalogue: {}\nPath: {}\nSuites: {}\nProduct shapes: {}\n\n",
            self.title,
            self.path.display(),
            self.suite_count,
            self.product_count
        );
        for entry in &self.entries {
            text.push_str(&entry.label);
            text.push('\n');
        }
        text
    }
}

#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
struct CatalogueManifest {
    schema_version: u32,
    kind: String,
    id: String,
    title: String,
    publication_boundary: String,
    publisher: Publisher,
    #[serde(default)]
    peers: Vec<PeerReference>,
    suites: Vec<SuiteReference>,
    semantic_concepts: Vec<SemanticConcept>,
}

#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
struct Publisher {
    name: String,
    short_name: String,
}

#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
struct PeerReference {
    id: String,
    title: String,
    bundle: String,
    namespace: String,
    suites: Vec<String>,
}

#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
struct SuiteReference {
    id: String,
    title: String,
    manifest: String,
}

#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
struct SemanticConcept {
    id: String,
    title: String,
    description: String,
    product_references: Vec<String>,
}

#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
struct SuiteManifest {
    schema_version: u32,
    kind: String,
    id: String,
    title: String,
    #[serde(default)]
    peer: Option<String>,
    publication_boundary: String,
    source_specification: SourceSpecification,
    default_publication_profile: PublicationProfile,
    categories: Vec<Category>,
}

#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
struct SourceSpecification {
    title: String,
    url: String,
}

#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
struct PublicationProfile {
    data_kind: String,
    data_format: String,
    source_delivery: String,
    publication_readiness: String,
}

#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
struct Category {
    id: String,
    title: String,
    products: Vec<Product>,
}

#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
struct Product {
    id: String,
    title: String,
    #[serde(default)]
    cadence: Option<String>,
    #[serde(default)]
    semantic_concept_id: Option<String>,
    #[serde(default)]
    value_type: Option<String>,
    #[serde(default)]
    semantic_measure: Option<String>,
    #[serde(default)]
    variants: Vec<ProductVariant>,
}

#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
struct ProductVariant {
    id: String,
    title: String,
    #[serde(default)]
    time_representation: Option<String>,
}

pub fn load(path: &Path) -> Result<SourceCatalogue, String> {
    let requested = std::path::absolute(path).map_err(|error| error.to_string())?;
    let index_path = if requested.is_dir() {
        requested.join("catalogue.json")
    } else {
        requested
    };
    let index_path = checked_regular_file(&index_path)?;
    let root = index_path
        .parent()
        .ok_or_else(|| "Source catalogue index has no parent directory".to_string())?
        .to_path_buf();
    let manifest: CatalogueManifest = read_json(&index_path)?;
    validate_header(
        manifest.schema_version,
        &manifest.kind,
        "feam.source-product-catalogue.v1",
        &manifest.publication_boundary,
        "source catalogue",
    )?;
    validate_id(&manifest.id, "catalogue id")?;
    if manifest.suites.is_empty() || manifest.suites.len() > MAX_SUITES {
        return Err(format!(
            "Source catalogue must contain 1..={MAX_SUITES} suites"
        ));
    }

    let mut suite_ids = HashSet::new();
    let mut loaded = Vec::with_capacity(manifest.suites.len());
    for suite in &manifest.suites {
        validate_id(&suite.id, "suite id")?;
        if !suite_ids.insert(suite.id.clone()) {
            return Err(format!("Duplicate source suite id {}", suite.id));
        }
        let relative = Path::new(&suite.manifest);
        if relative.components().count() != 1
            || !matches!(relative.components().next(), Some(Component::Normal(_)))
        {
            return Err(format!(
                "Source suite manifest path must be one filename: {}",
                suite.manifest
            ));
        }
        let suite_path = checked_regular_file(&root.join(relative))?;
        if suite_path.parent() != Some(root.as_path()) {
            return Err(format!(
                "Source suite manifest escapes the catalogue directory: {}",
                suite.manifest
            ));
        }
        let suite_manifest: SuiteManifest = read_json(&suite_path)?;
        validate_suite(&suite_manifest, &suite.id)?;
        loaded.push((suite, suite_manifest));
    }

    let mut products: HashMap<String, Option<String>> = HashMap::new();
    for (suite_ref, suite) in &loaded {
        for category in &suite.categories {
            for product in &category.products {
                let reference = format!("{}/{}/{}", suite_ref.id, category.id, product.id);
                if products
                    .insert(reference.clone(), product.semantic_concept_id.clone())
                    .is_some()
                {
                    return Err(format!("Duplicate source product reference {reference}"));
                }
            }
        }
    }

    let mut concepts = HashMap::new();
    for concept in &manifest.semantic_concepts {
        validate_id(&concept.id, "semantic concept id")?;
        if concepts.insert(concept.id.clone(), concept).is_some() {
            return Err(format!("Duplicate semantic concept {}", concept.id));
        }
        for reference in &concept.product_references {
            match products.get(reference) {
                Some(Some(product_concept)) if product_concept == &concept.id => {}
                Some(_) => {
                    return Err(format!(
                        "Semantic concept {} disagrees with product {reference}",
                        concept.id
                    ));
                }
                None => {
                    return Err(format!(
                        "Semantic concept {} references unknown product {reference}",
                        concept.id
                    ));
                }
            }
        }
    }
    for (reference, concept) in &products {
        if let Some(concept_id) = concept {
            let Some(concept) = concepts.get(concept_id) else {
                return Err(format!(
                    "Source product {reference} names unknown semantic concept {concept_id}"
                ));
            };
            if !concept
                .product_references
                .iter()
                .any(|candidate| candidate == reference)
            {
                return Err(format!(
                    "Semantic concept {concept_id} omits product {reference}"
                ));
            }
        }
    }

    let mut entries = Vec::new();
    let concepts_summary = if manifest.semantic_concepts.is_empty() {
        "none".to_string()
    } else {
        manifest
            .semantic_concepts
            .iter()
            .map(|concept| format!("{} ({})", concept.title, concept.id))
            .collect::<Vec<_>>()
            .join("\n")
    };
    let peers_summary = manifest
        .peers
        .iter()
        .map(|peer| {
            format!(
                "{} ({}) → bundle={}, namespace={}, suites={}",
                peer.title,
                peer.id,
                peer.bundle,
                peer.namespace,
                peer.suites.join(", ")
            )
        })
        .collect::<Vec<_>>()
        .join("\n");
    entries.push(SourceCatalogueEntry {
        label: manifest.title.clone(),
        reference: manifest.id.clone(),
        detail: format!(
            "Source-product catalogue\n{}\n\nPublisher: {} ({})\nBoundary: catalogue only; no assets are published by this view.\nPeers\n{}\n\nSuites: {}\n\nShared scientific concepts\n{}",
            manifest.title,
            manifest.publisher.name,
            manifest.publisher.short_name,
            peers_summary,
            manifest.suites.len(),
            concepts_summary
        ),
    });

    let mut product_count = 0;
    for (suite_ref, suite) in loaded {
        entries.push(SourceCatalogueEntry {
            label: format!("  {}", suite_ref.title),
            reference: suite.id.clone(),
            detail: format!(
                "Source suite: {}\nID: {}\nPeer: {}\nSpecification: {}\n{}\n\nExpected publication profile\nKind: {}\nFormat: {}\nSource delivery: {}\nReadiness: {}\n\nThis is descriptive metadata, not a FEAM serving manifest.",
                suite.title,
                suite.id,
                suite.peer.as_deref().unwrap_or("unspecified"),
                suite.source_specification.title,
                suite.source_specification.url,
                suite.default_publication_profile.data_kind,
                suite.default_publication_profile.data_format,
                suite.default_publication_profile.source_delivery,
                suite.default_publication_profile.publication_readiness
            ),
        });
        for category in suite.categories {
            let category_reference = format!("{}/{}", suite.id, category.id);
            entries.push(SourceCatalogueEntry {
                label: format!("    {}", category.title),
                reference: category_reference.clone(),
                detail: format!(
                    "Source category: {}\nReference: {}\nProduct shapes: {}",
                    category.title,
                    category_reference,
                    category.products.len()
                ),
            });
            for product in category.products {
                product_count += 1;
                let reference = format!("{}/{}/{}", suite.id, category.id, product.id);
                let concept_detail = product
                    .semantic_concept_id
                    .as_ref()
                    .and_then(|concept_id| concepts.get(concept_id))
                    .map(|concept| {
                        format!(
                            "\nShared concept: {} ({})\n{}\nRelated product shapes:\n{}",
                            concept.title,
                            concept.id,
                            concept.description,
                            concept.product_references.join("\n")
                        )
                    })
                    .unwrap_or_default();
                let optional = [
                    product
                        .cadence
                        .as_ref()
                        .map(|value| format!("Cadence: {value}")),
                    product
                        .value_type
                        .as_ref()
                        .map(|value| format!("Value type: {value}")),
                    product
                        .semantic_measure
                        .as_ref()
                        .map(|value| format!("Semantic measure: {value}")),
                ]
                .into_iter()
                .flatten()
                .collect::<Vec<_>>()
                .join("\n");
                let optional = if optional.is_empty() {
                    String::new()
                } else {
                    format!("\n{optional}")
                };
                entries.push(SourceCatalogueEntry {
                    label: format!("      {}", product.title),
                    reference: reference.clone(),
                    detail: format!(
                        "Source product shape: {}\nReference: {}{}{}\n\nNo source object, version, checksum, bounds, or asset path is implied.",
                        product.title, reference, optional, concept_detail
                    ),
                });
                for variant in product.variants {
                    let variant_reference = format!("{reference}/{}", variant.id);
                    let time = variant
                        .time_representation
                        .map(|value| format!("\nTime representation: {value}"))
                        .unwrap_or_default();
                    entries.push(SourceCatalogueEntry {
                        label: format!("        {}", variant.title),
                        reference: variant_reference.clone(),
                        detail: format!(
                            "Source product variant: {}\nReference: {}{}\n\nThis remains a catalogue-only variant.",
                            variant.title, variant_reference, time
                        ),
                    });
                }
            }
        }
    }

    Ok(SourceCatalogue {
        path: index_path,
        title: manifest.title,
        entries,
        suite_count: manifest.suites.len(),
        product_count,
    })
}

fn checked_regular_file(path: &Path) -> Result<PathBuf, String> {
    let metadata = fs::symlink_metadata(path).map_err(|error| {
        format!(
            "Cannot inspect source catalogue file {}: {error}",
            path.display()
        )
    })?;
    if metadata.file_type().is_symlink() || !metadata.is_file() {
        return Err(format!(
            "Source catalogue path must be a regular, non-symlink file: {}",
            path.display()
        ));
    }
    if metadata.len() > MAX_FILE_BYTES {
        return Err(format!(
            "Source catalogue file exceeds {MAX_FILE_BYTES} bytes: {}",
            path.display()
        ));
    }
    fs::canonicalize(path)
        .map_err(|error| format!("Cannot canonicalize source catalogue file: {error}"))
}

fn read_json<T: for<'de> Deserialize<'de>>(path: &Path) -> Result<T, String> {
    let mut bytes = Vec::new();
    fs::File::open(path)
        .map_err(|error| error.to_string())?
        .take(MAX_FILE_BYTES + 1)
        .read_to_end(&mut bytes)
        .map_err(|error| error.to_string())?;
    if bytes.len() as u64 > MAX_FILE_BYTES {
        return Err(format!(
            "Source catalogue file exceeds {MAX_FILE_BYTES} bytes: {}",
            path.display()
        ));
    }
    serde_json::from_slice(&bytes).map_err(|error| {
        format!(
            "Invalid source catalogue JSON in {}: {error}",
            path.display()
        )
    })
}

fn validate_header(
    version: u32,
    kind: &str,
    expected_kind: &str,
    boundary: &str,
    label: &str,
) -> Result<(), String> {
    if version != 1 || kind != expected_kind {
        return Err(format!("Unsupported {label} schema or kind"));
    }
    if boundary != "catalogue_only" {
        return Err(format!(
            "{label} must declare publication_boundary catalogue_only"
        ));
    }
    Ok(())
}

fn validate_suite(suite: &SuiteManifest, expected_id: &str) -> Result<(), String> {
    validate_header(
        suite.schema_version,
        &suite.kind,
        "feam.source-product-suite.v1",
        &suite.publication_boundary,
        "source suite",
    )?;
    validate_id(&suite.id, "source suite id")?;
    if suite.id != expected_id {
        return Err(format!(
            "Source suite id {} does not match index id {expected_id}",
            suite.id
        ));
    }
    if suite.categories.is_empty() || suite.categories.len() > MAX_CATEGORIES_PER_SUITE {
        return Err(format!(
            "Source suite {} must contain 1..={MAX_CATEGORIES_PER_SUITE} categories",
            suite.id
        ));
    }
    let mut category_ids = HashSet::new();
    for category in &suite.categories {
        validate_id(&category.id, "source category id")?;
        if !category_ids.insert(category.id.clone()) {
            return Err(format!(
                "Duplicate source category id {}/{}",
                suite.id, category.id
            ));
        }
        if category.products.is_empty() || category.products.len() > MAX_PRODUCTS_PER_CATEGORY {
            return Err(format!(
                "Source category {}/{} must contain 1..={MAX_PRODUCTS_PER_CATEGORY} products",
                suite.id, category.id
            ));
        }
        let mut product_ids = HashSet::new();
        for product in &category.products {
            validate_id(&product.id, "source product id")?;
            if !product_ids.insert(product.id.clone()) {
                return Err(format!(
                    "Duplicate source product id {}/{}/{}",
                    suite.id, category.id, product.id
                ));
            }
            if product.variants.len() > MAX_VARIANTS_PER_PRODUCT {
                return Err(format!(
                    "Source product {}/{}/{} exceeds {MAX_VARIANTS_PER_PRODUCT} variants",
                    suite.id, category.id, product.id
                ));
            }
            let mut variant_ids = HashSet::new();
            for variant in &product.variants {
                validate_id(&variant.id, "source product variant id")?;
                if !variant_ids.insert(variant.id.clone()) {
                    return Err(format!(
                        "Duplicate source product variant id {}/{}/{}/{}",
                        suite.id, category.id, product.id, variant.id
                    ));
                }
            }
        }
    }
    Ok(())
}

fn validate_id(value: &str, label: &str) -> Result<(), String> {
    let valid = !value.is_empty()
        && value.len() <= 128
        && value.starts_with(|character: char| character.is_ascii_lowercase())
        && value
            .bytes()
            .all(|byte| byte.is_ascii_lowercase() || byte.is_ascii_digit() || byte == b'-');
    if valid {
        Ok(())
    } else {
        Err(format!("Invalid {label}: {value}"))
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn repository_catalogue() -> PathBuf {
        Path::new(env!("CARGO_MANIFEST_DIR")).join("../../web_demo/catalogue/aafc/catalogue.json")
    }

    #[test]
    fn repository_aafc_catalogue_loads_as_descriptive_metadata() {
        let catalogue = load(&repository_catalogue()).unwrap();
        assert_eq!(catalogue.suite_count, 3);
        assert_eq!(catalogue.product_count, 41);
        assert_eq!(catalogue.entries.len(), 52);
        let spi = catalogue
            .entries
            .iter()
            .find(|entry| {
                entry.reference == "agroclimate/drought-indicators/standardized-precipitation-index"
            })
            .unwrap();
        assert!(spi.detail.contains("Related product shapes"));
        assert!(spi.detail.contains("nasdi/precipitation"));
        assert!(spi.detail.contains("No source object"));
    }

    #[test]
    fn suite_paths_cannot_escape_the_catalogue_directory() {
        let temp = tempfile::tempdir().unwrap();
        fs::write(
            temp.path().join("catalogue.json"),
            serde_json::json!({
                "schema_version": 1,
                "kind": "feam.source-product-catalogue.v1",
                "id": "test",
                "title": "Test",
                "publication_boundary": "catalogue_only",
                "publisher": {"name": "Test", "short_name": "T"},
                "suites": [{"id": "suite", "title": "Suite", "manifest": "../suite.json"}],
                "semantic_concepts": []
            })
            .to_string(),
        )
        .unwrap();
        let error = load(&temp.path().join("catalogue.json")).unwrap_err();
        assert!(error.contains("one filename"));
    }

    #[test]
    fn source_catalogue_cannot_claim_publication_authority() {
        let temp = tempfile::tempdir().unwrap();
        fs::write(
            temp.path().join("catalogue.json"),
            serde_json::json!({
                "schema_version": 1,
                "kind": "feam.source-product-catalogue.v1",
                "id": "test",
                "title": "Test",
                "publication_boundary": "serving",
                "publisher": {"name": "Test", "short_name": "T"},
                "suites": [{"id": "suite", "title": "Suite", "manifest": "suite.json"}],
                "semantic_concepts": []
            })
            .to_string(),
        )
        .unwrap();
        let error = load(&temp.path().join("catalogue.json")).unwrap_err();
        assert!(error.contains("publication_boundary catalogue_only"));
    }
}
