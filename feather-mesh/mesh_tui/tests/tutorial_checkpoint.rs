use tempfile::tempdir;
use std::path::PathBuf;

#[test]
fn checkpoint_saved_on_advance() {
    let dir = tempdir().unwrap();
    let checkpoint_path = dir.path().join("tutorial.json");
    let mut ctrl = mesh_tui::tutorial::TutorialController::new(checkpoint_path.clone());
    ctrl.checkpoint.project_root = Some(PathBuf::from("/tmp"));
    ctrl.advance();
    ctrl.save_checkpoint().unwrap();
    let contents = std::fs::read_to_string(&checkpoint_path).unwrap();
    assert!(contents.contains("ProjectBinding") || contents.contains("project_root"));
}
