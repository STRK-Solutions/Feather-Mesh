#[path = "../../mesh_core/tests/support/mod.rs"]
mod fixtures;
use mesh_agent::provider::{ModelRequest, ProviderToolCall};
use mesh_agent::*;
use mesh_core::peer::*;
use mesh_core::services::interactive_operations::*;
use serde_json::json;
use std::sync::{Arc, Mutex};

fn profile() -> AgentProfile {
    serde_json::from_value(json!({"backend":"router","base_url":"https://openrouter.ai/api/v1","model":"fixture","api_key_env":"UNUSED","context_policy":"metadata-only","allow_user_text":true,"max_context_chars":32000})).unwrap()
}
fn fixture() -> (tempfile::TempDir, Project) {
    let tmp = tempfile::tempdir().unwrap();
    let project = Project::init(
        tmp.path().join("provider"),
        "climate".into(),
        Some("serving".into()),
    )
    .unwrap();
    let req = fixtures::table_request();
    for a in &req.assets {
        let p = project.serving_root().unwrap().join(&a.path);
        std::fs::create_dir_all(p.parent().unwrap()).unwrap();
        fixtures::write_parquet(&p, &[1, 2]);
    }
    publish(&project, &req).unwrap();
    let mut req = req;
    req.version = "v2".into();
    publish(&project, &req).unwrap();
    (tmp, project)
}
fn call(name: &str, args: serde_json::Value, id: &str) -> ProviderEvent {
    ProviderEvent::ToolCall(ProviderToolCall {
        id: id.into(),
        name: name.into(),
        arguments: args,
    })
}
#[derive(Clone)]
struct Capture {
    fake: FakeProvider,
    requests: Arc<Mutex<Vec<ModelRequest>>>,
}
impl ModelProvider for Capture {
    fn stream(
        &mut self,
        r: ModelRequest,
        c: &CancellationToken,
        e: &mut dyn FnMut(ProviderEvent),
    ) -> Result<(), ProviderError> {
        self.requests.lock().unwrap().push(r.clone());
        self.fake.stream(r, c, e)
    }
}
#[test]
fn clarification_retains_context_and_pinned_resolution_evidence() {
    let (_tmp, project) = fixture();
    let requests = Arc::new(Mutex::new(Vec::new()));
    let provider = Capture {
        requests: requests.clone(),
        fake: FakeProvider::new([
            Ok(vec![call(
                "catalog.search",
                json!({"text":"observations"}),
                "search",
            )]),
            Ok(vec![ProviderEvent::TextDelta("Choose v1 or v2?".into())]),
            Ok(vec![call(
                "product.resolve",
                json!({"reference":"product://climate/observations","version":"v2"}),
                "resolve",
            )]),
            Ok(vec![ProviderEvent::TextDelta(
                "Resolved v2 using the registered inventory.".into(),
            )]),
        ]),
    };
    let mut harness = AgentHarness::new(profile(), provider, project.root());
    assert_eq!(
        harness.run("Find observations").state,
        AgentRunState::AwaitingClarification
    );
    let run = harness.run("Choose v2");
    assert_eq!(run.state, AgentRunState::Complete);
    assert!(
        matches!(&run.tools[0],ToolExecution::Read{result,..} if result["version"]=="v2" && result["assets"].as_array().unwrap().len()==2)
    );
    let captured = serde_json::to_string(&*requests.lock().unwrap()).unwrap();
    assert!(captured.contains("Choose v2"));
    assert!(captured.contains("Choose v1 or v2?"));
    assert!(!captured.contains(&project.root().display().to_string()));
}
#[test]
fn local_integrity_consent_cannot_be_asserted_by_provider() {
    let (_tmp, project) = fixture();
    let mut executor = ToolExecutor::new(project.root());
    let proposal = |id: &str, args| ProviderToolCall {
        id: id.into(),
        name: "product.resolve".into(),
        arguments: args,
    };
    let args = json!({"reference":"product://climate/observations","version":"v1","verify_integrity":true});
    assert!(
        matches!(executor.execute(proposal("no-consent",args.clone())),ToolExecution::Rejected{kind,..} if kind=="integrity_consent_required")
    );
    let mut forged = args.clone();
    forged["integrity_consent"] = json!(true);
    assert!(
        matches!(executor.execute(proposal("forged",forged)),ToolExecution::Rejected{kind,..} if kind=="bad_arguments")
    );
    executor.grant_integrity_consent("product://climate/observations".into(), "v1".into());
    assert!(
        matches!(executor.execute(proposal("local",args.clone())),ToolExecution::Read{result,..} if result["integrity_verified"]==true)
    );
    assert!(matches!(
        executor.execute(proposal("consumed", args)),
        ToolExecution::Rejected { .. }
    ));
}
#[test]
fn review_denial_and_authoritative_result_are_correlated_without_replay() {
    let (tmp, project) = fixture();
    let output = tmp.path().join("stage");
    let prepared = prepare_stage(
        project.root(),
        "product://climate/observations",
        "v1",
        &output,
        false,
    )
    .unwrap();
    let id = prepared.operation_id.clone();
    let fake = FakeProvider::new([
        Ok(vec![call("product.stage", json!({"handle":id}), "stage")]),
        Ok(vec![ProviderEvent::TextDelta(
            "The local user denied this copy.".into(),
        )]),
    ]);
    let mut harness = AgentHarness::new(profile(), fake, project.root());
    harness
        .executor_mut()
        .register_operation(ConfirmableOperation::Stage(Box::new(prepared)));
    let run = harness.run("Propose the selected stage");
    assert!(matches!(run.state, AgentRunState::AwaitingReview { .. }));
    assert!(!output.exists());
    harness
        .record_review_outcome(
            &id,
            json!({"operation_id":id,"status":"denied","destination":"/private/hidden"}),
        )
        .unwrap();
    assert_eq!(harness.run("").state, AgentRunState::Complete);
    assert!(!output.exists());
    assert!(
        harness
            .record_review_outcome(&id, json!({"state":"committed"}))
            .is_err()
    );
    assert!(
        harness
            .executor_mut()
            .take_prepared_operation(&id)
            .is_none()
    );
}
#[test]
fn late_cancel_and_project_switch_expire_handles() {
    struct CancelAfter;
    impl ModelProvider for CancelAfter {
        fn stream(
            &mut self,
            _: ModelRequest,
            c: &CancellationToken,
            e: &mut dyn FnMut(ProviderEvent),
        ) -> Result<(), ProviderError> {
            e(call("catalog.refresh", json!({}), "late"));
            c.cancel();
            Ok(())
        }
    }
    let (_tmp, project) = fixture();
    let mut h = AgentHarness::new(profile(), CancelAfter, project.root());
    let run = h.run("Refresh");
    assert_eq!(run.state, AgentRunState::Stopped);
    assert!(run.tools.is_empty());
    let mut e = ToolExecutor::new(project.root());
    let handle = e
        .prepare_withdrawal_for_review("product://climate/observations", "v1", "superseded")
        .unwrap();
    e.reset_project(project.root());
    assert!(
        matches!(e.execute(ProviderToolCall{id:"new".into(),name:"product.withdraw".into(),arguments:json!({"handle":handle})}),ToolExecution::Rejected{kind,..} if kind=="unknown_handle")
    );
}
#[test]
fn disclosure_channels_and_unknown_cost_stop_before_next_request() {
    let p = profile();
    let summary = mesh_agent::disclosure::tool_summary(
        &json!({"result":{"reference":"product://climate/observations","description":"secret /private/hidden","contact":"private@example.test","assets":[{"id":"data","project_access_path":"/private/file","size":1}],"coverage":[{"namespace":"climate","error":"secret /private/failure"}]}}),
        &p,
    );
    let text = summary.to_string();
    assert!(!text.contains("private"));
    assert!(text.contains("product://climate/observations"));
    let mut p = p;
    p.max_cost_usd = Some(1.0);
    let requests = Arc::new(Mutex::new(Vec::new()));
    let provider = Capture {
        requests: requests.clone(),
        fake: FakeProvider::new([Ok(vec![call(
            "help.lookup",
            json!({"topic":"resolve"}),
            "help",
        )])]),
    };
    let mut h = AgentHarness::new(p, provider, ".");
    let run = h.run("Explain direct reads /private/user-path");
    assert!(matches!(run.state,AgentRunState::Failed{kind} if kind=="cost_unknown"));
    assert_eq!(requests.lock().unwrap().len(), 1);
}
#[test]
fn full_context_and_output_are_bounded() {
    let mut p = profile();
    p.max_context_chars = 32;
    let mut h = AgentHarness::new(p, FakeProvider::new([]), ".");
    assert!(matches!(h.run("help").state,AgentRunState::Failed{kind} if kind=="context_limit"));
    let mut p = profile();
    p.max_response_chars = 8;
    let mut h = AgentHarness::new(
        p,
        FakeProvider::new([Ok(vec![ProviderEvent::TextDelta(
            "far too much output".into(),
        )])]),
        ".",
    );
    assert!(matches!(h.run("help").state,AgentRunState::Failed{kind} if kind=="output_limit"));
}

#[test]
fn complete_batches_are_serialized_and_every_call_is_correlated() {
    let (_tmp, project) = fixture();
    let requests = Arc::new(Mutex::new(Vec::new()));
    let provider = Capture {
        requests: requests.clone(),
        fake: FakeProvider::new([
            Ok(vec![
                call("help.lookup", json!({"topic":"resolve"}), "help"),
                call(
                    "product.inspect",
                    json!({"reference":"product://climate/observations","version":"v1"}),
                    "inspect",
                ),
            ]),
            Ok(vec![ProviderEvent::TextDelta(
                "Grounded in both results.".into(),
            )]),
        ]),
    };
    let mut h = AgentHarness::new(profile(), provider, project.root());
    let result = h.run("Explain this pinned version");
    assert_eq!(result.tools.len(), 2);
    assert_eq!(result.state, AgentRunState::Complete);
    let captured = requests.lock().unwrap();
    let second = &captured[1];
    let ids: Vec<_> = second
        .messages
        .iter()
        .filter_map(|m| m.tool_call_id.as_deref())
        .collect();
    assert_eq!(ids, ["help", "inspect"]);
}
#[test]
fn a_mutation_proposal_stops_the_rest_of_a_complete_batch_for_local_review() {
    let (tmp, project) = fixture();
    let prepared = prepare_stage(
        project.root(),
        "product://climate/observations",
        "v1",
        tmp.path().join("out"),
        false,
    )
    .unwrap();
    let id = prepared.operation_id.clone();
    let fake = FakeProvider::new([Ok(vec![
        call("product.stage", json!({"handle":id}), "stage"),
        call("catalog.refresh", json!({}), "after-review"),
    ])]);
    let mut h = AgentHarness::new(profile(), fake, project.root());
    h.executor_mut()
        .register_operation(ConfirmableOperation::Stage(Box::new(prepared)));
    let run = h.run("Propose staging");
    assert!(matches!(run.state, AgentRunState::AwaitingReview { .. }));
    assert!(
        matches!(&run.tools[1],ToolExecution::Rejected{kind,..} if kind=="local_review_pending")
    );
    assert!(!tmp.path().join("out").exists());
}

#[test]
fn local_handle_context_filters_withdrawal_reason_and_replaces_old_handles() {
    let (_tmp, project) = fixture();
    let requests = Arc::new(Mutex::new(Vec::new()));
    let provider = Capture {
        requests: requests.clone(),
        fake: FakeProvider::new([
            Ok(vec![ProviderEvent::TextDelta(
                "Ready for local review.".into(),
            )]),
            Ok(vec![ProviderEvent::TextDelta(
                "Ready for the newly selected review.".into(),
            )]),
        ]),
    };
    let mut h = AgentHarness::new(profile(), provider, project.root());
    let old = h
        .executor_mut()
        .prepare_withdrawal_for_review(
            "product://climate/observations",
            "v1",
            "private-contact@example.test /private/reason",
        )
        .unwrap();
    h.run("Describe the selected operation");
    h.executor_mut().invalidate_handles();
    let new = h
        .executor_mut()
        .prepare_withdrawal_for_review("product://climate/observations", "v2", "superseded")
        .unwrap();
    h.run("Describe the newly selected operation");
    let captured = requests.lock().unwrap();
    let first = serde_json::to_string(&captured[0]).unwrap();
    assert!(first.contains(&old));
    assert!(first.contains("product.withdraw"));
    assert!(first.contains("locally_prepared_awaiting_review"));
    assert!(!first.contains("private-contact"));
    assert!(!first.contains("/private/reason"));
    let second = serde_json::to_string(&captured[1]).unwrap();
    assert!(!second.contains(&old));
    assert!(second.contains(&new));
}
