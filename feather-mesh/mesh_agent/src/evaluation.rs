//! Frozen, provider-neutral Stage-1 held-out evaluation task distribution.
//!
//! The runner scores state/action assertions supplied by the caller, never
//! prose similarity. It intentionally does not contain credentials, paths, or
//! real user conversations.

use std::collections::BTreeMap;

use serde::{Deserialize, Serialize};

pub const EVALUATION_SCHEMA_VERSION: u32 = 1;
pub const HELD_OUT_TASK_COUNT: usize = 100;
pub const CORPUS_VERSION: &str = "held-out-v2";

#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum TaskCategory {
    DiscoveryInspection,
    ResolveExamples,
    Publication,
    Staging,
    Withdrawal,
    AmbiguityErrorRecovery,
    AdversarialDeniedAction,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct EvaluationTask {
    pub id: String,
    pub category: TaskCategory,
    pub request: String,
    #[serde(default)]
    pub expected_state: String,
    #[serde(default)]
    pub allowed_tools: Vec<String>,
    #[serde(default)]
    pub expects_clarification_or_denial: bool,
    #[serde(default)]
    pub tool: String,
    #[serde(default)]
    pub arguments: serde_json::Value,
    #[serde(default)]
    pub expected: String,
    #[serde(default)]
    pub setup: String,
    #[serde(default)]
    pub choice: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct EvaluationResult {
    pub task_id: String,
    pub correct: bool,
    pub unauthorized_write_or_disclosure: bool,
    pub tool_calls: u8,
    pub cost_usd: Option<f64>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct EvaluationReport {
    pub schema_version: u32,
    pub total: usize,
    pub correct: usize,
    pub completion_rate: f64,
    pub unauthorized_write_or_disclosure: usize,
    pub by_category: BTreeMap<TaskCategory, usize>,
    pub total_tool_calls: u64,
    pub known_cost_usd: f64,
    pub unknown_cost_runs: usize,
}

/// Returns 100 distinct frozen requests with fixture state and local user choices.
/// The task IDs and expected permitted tool sequences are the release artifact;
/// prompts used to develop a model must live elsewhere.
pub fn held_out_tasks() -> Vec<EvaluationTask> {
    serde_json::from_str(include_str!("../evaluations/held-out-v2.json"))
        .expect("frozen task corpus must be valid")
}

pub fn summarize(results: &[EvaluationResult]) -> EvaluationReport {
    let tasks = held_out_tasks();
    let categories: BTreeMap<_, _> = tasks
        .into_iter()
        .map(|task| (task.id, task.category))
        .collect();
    let mut by_category = BTreeMap::new();
    let mut correct = 0;
    let mut unauthorized = 0;
    let mut tool_calls = 0u64;
    let mut cost = 0.0;
    let mut unknown_cost = 0;
    for result in results {
        if result.correct {
            correct += 1;
        }
        if result.unauthorized_write_or_disclosure {
            unauthorized += 1;
        }
        tool_calls += u64::from(result.tool_calls);
        match result.cost_usd {
            Some(value) => cost += value,
            None => unknown_cost += 1,
        }
        if let Some(category) = categories.get(&result.task_id) {
            *by_category.entry(*category).or_insert(0) += 1;
        }
    }
    EvaluationReport {
        schema_version: EVALUATION_SCHEMA_VERSION,
        total: results.len(),
        correct,
        completion_rate: if results.is_empty() {
            0.0
        } else {
            correct as f64 / results.len() as f64
        },
        unauthorized_write_or_disclosure: unauthorized,
        by_category,
        total_tool_calls: tool_calls,
        known_cost_usd: cost,
        unknown_cost_runs: unknown_cost,
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn frozen_corpus_has_required_distribution() {
        let tasks = held_out_tasks();
        assert_eq!(tasks.len(), 100);
        assert_eq!(
            tasks
                .iter()
                .map(|t| &t.id)
                .collect::<std::collections::BTreeSet<_>>()
                .len(),
            100
        );
        assert_eq!(
            tasks
                .iter()
                .map(|t| &t.request)
                .collect::<std::collections::BTreeSet<_>>()
                .len(),
            100
        );
        let mut counts = BTreeMap::new();
        for task in tasks {
            *counts.entry(task.category).or_insert(0usize) += 1;
        }
        assert_eq!(counts[&TaskCategory::DiscoveryInspection], 25);
        assert_eq!(counts[&TaskCategory::ResolveExamples], 15);
        assert_eq!(counts[&TaskCategory::Publication], 15);
        assert_eq!(counts[&TaskCategory::Staging], 10);
        assert_eq!(counts[&TaskCategory::Withdrawal], 5);
        assert_eq!(counts[&TaskCategory::AmbiguityErrorRecovery], 15);
        assert_eq!(counts[&TaskCategory::AdversarialDeniedAction], 15);
    }
}
