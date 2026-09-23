#![allow(dead_code)]
use mesh_core::peer::*;
use parquet::data_type::Int32Type;
use parquet::file::writer::SerializedFileWriter;
use parquet::schema::parser::parse_message_type;
use std::collections::BTreeMap;
use std::fs::{self, File};
use std::path::{Path, PathBuf};
use std::sync::Arc;
use tiff::encoder::{TiffEncoder, colortype};
use tiff::tags::Tag;
pub fn write_parquet(path: &Path, values: &[i32]) {
    let schema = Arc::new(
        parse_message_type(
            "message observations { REQUIRED INT32 station_id; REQUIRED INT32 temperature; }",
        )
        .unwrap(),
    );
    let file = File::create(path).unwrap();
    let mut writer = SerializedFileWriter::new(file, schema, Default::default()).unwrap();
    let mut row_group = writer.next_row_group().unwrap();
    let mut station = row_group.next_column().unwrap().unwrap();
    station
        .typed::<Int32Type>()
        .write_batch(values, None, None)
        .unwrap();
    station.close().unwrap();
    let mut temperature = row_group.next_column().unwrap().unwrap();
    temperature
        .typed::<Int32Type>()
        .write_batch(
            &values.iter().map(|value| value + 10).collect::<Vec<_>>(),
            None,
            None,
        )
        .unwrap();
    temperature.close().unwrap();
    row_group.close().unwrap();
    writer.close().unwrap();
}

pub fn write_geotiff(path: &Path) {
    let file = File::create(path).unwrap();
    let mut encoder = TiffEncoder::new(file).unwrap();
    let mut image = encoder.new_image::<colortype::Gray8>(2, 2).unwrap();
    image
        .encoder()
        .write_tag(
            Tag::GeoKeyDirectoryTag,
            &[1_u16, 1, 0, 1, 1024, 0, 1, 2][..],
        )
        .unwrap();
    image
        .encoder()
        .write_tag(Tag::ModelPixelScaleTag, &[1_f64, 1_f64, 0_f64][..])
        .unwrap();
    image.write_data(&[7_u8, 8, 9, 10]).unwrap();
}

pub fn table_request() -> PublicationRequest {
    PublicationRequest {
        schema_version: 1,
        namespace: "climate".into(),
        product_id: "observations".into(),
        name: "Daily observations".into(),
        version: "v1".into(),
        data_kind: DataKind::Table,
        data_format: DataFormat::Parquet,
        description: "Small registered observations fixture".into(),
        intended_use: "test direct reads".into(),
        limitations: "none".into(),
        owner_team: "climate".into(),
        producer: "Climate Lab".into(),
        contact: "climate@example.test".into(),
        usage_policy: "internal".into(),
        classification: "internal".into(),
        quality: "production".into(),
        assets: vec![
            DeclaredAsset {
                id: "part-000".into(),
                path: PathBuf::from("datasets/observations/v1/part-000.parquet"),
                role: "data".into(),
                media_type: "application/vnd.apache.parquet".into(),
                digest_opt_out: false,
            },
            DeclaredAsset {
                id: "part-001".into(),
                path: PathBuf::from("datasets/observations/v1/part-001.parquet"),
                role: "data".into(),
                media_type: "application/vnd.apache.parquet".into(),
                digest_opt_out: false,
            },
        ],
        lineage: vec![],
        table: Some(TablePublication {
            column_meanings: BTreeMap::from([
                ("station_id".into(), "station identifier".into()),
                ("temperature".into(), "daily temperature".into()),
            ]),
            column_units: BTreeMap::from([
                ("station_id".into(), "not_applicable".into()),
                ("temperature".into(), "celsius".into()),
            ]),
            partition_columns: vec![],
        }),
        raster: None,
    }
}

pub fn configure_client(client: &Project, provider_serving: &Path) {
    let peer_dir = client.root().join("peers");
    fs::create_dir_all(&peer_dir).unwrap();
    #[cfg(unix)]
    std::os::unix::fs::symlink(provider_serving, peer_dir.join("nearby-climate")).unwrap();
    let mut config = client.config().clone();
    config.peers.push(PeerRouteConfig {
        alias: "nearby-climate".into(),
        namespace: "climate".into(),
        path: PathBuf::from("peers/nearby-climate"),
    });
    fs::write(
        client.config_path(),
        toml::to_string_pretty(&config).unwrap(),
    )
    .unwrap();
}
