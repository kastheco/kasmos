//! Dependency graph for work packages.
//!
//! This module provides a minimal dependency graph type that tracks work package
//! dependencies and allows efficient querying of dependency satisfaction and dependents.

use std::collections::{HashMap, HashSet};

/// A minimal dependency graph for work packages.
///
/// Tracks both forward dependencies (what a WP depends on) and reverse dependencies
/// (what depends on a WP) for efficient querying.
#[derive(Debug, Clone)]
pub struct DependencyGraph {
    /// Map from WP ID to its dependencies.
    pub dependencies: HashMap<String, Vec<String>>,

    /// Map from WP ID to its dependents (reverse dependencies).
    pub dependents: HashMap<String, Vec<String>>,
}

impl DependencyGraph {
    /// Create a new dependency graph from a list of work packages.
    ///
    /// # Arguments
    /// * `work_packages` - List of work packages with their dependencies
    pub fn new(work_packages: &[crate::types::WorkPackage]) -> Self {
        let mut dependencies = HashMap::new();
        let mut dependents: HashMap<String, Vec<String>> = HashMap::new();

        // Build forward dependencies
        for wp in work_packages {
            dependencies.insert(wp.id.clone(), wp.dependencies.clone());

            // Initialize dependents map for this WP
            dependents.entry(wp.id.clone()).or_insert_with(Vec::new);

            // Build reverse dependencies
            for dep in &wp.dependencies {
                dependents
                    .entry(dep.clone())
                    .or_insert_with(Vec::new)
                    .push(wp.id.clone());
            }
        }

        Self {
            dependencies,
            dependents,
        }
    }

    /// Check if all dependencies of a work package are satisfied.
    ///
    /// # Arguments
    /// * `wp_id` - The work package ID to check
    /// * `completed` - Set of completed work package IDs
    ///
    /// # Returns
    /// `true` if all dependencies are in the completed set, `false` otherwise
    pub fn deps_satisfied(&self, wp_id: &str, completed: &HashSet<String>) -> bool {
        self.dependencies
            .get(wp_id)
            .map(|deps| deps.iter().all(|dep| completed.contains(dep)))
            .unwrap_or(true)
    }

    /// Get the direct dependents of a work package.
    ///
    /// # Arguments
    /// * `wp_id` - The work package ID
    ///
    /// # Returns
    /// A vector of work package IDs that directly depend on this WP
    pub fn get_dependents(&self, wp_id: &str) -> Vec<String> {
        self.dependents.get(wp_id).cloned().unwrap_or_default()
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::types::{WPState, WorkPackage};

    fn create_test_wp(id: &str, deps: Vec<&str>) -> WorkPackage {
        WorkPackage {
            id: id.to_string(),
            title: format!("WP {}", id),
            state: WPState::Pending,
            dependencies: deps.iter().map(|s| s.to_string()).collect(),
            wave: 0,
            pane_id: None,
            pane_name: format!("wp_{}", id.to_lowercase()),
            worktree_path: None,
            prompt_path: None,
            started_at: None,
            completed_at: None,
            completion_method: None,
            failure_count: 0,
        }
    }

    #[test]
    fn test_graph_creation() {
        let wps = vec![
            create_test_wp("WP01", vec![]),
            create_test_wp("WP02", vec!["WP01"]),
            create_test_wp("WP03", vec!["WP01", "WP02"]),
        ];

        let graph = DependencyGraph::new(&wps);

        assert_eq!(
            graph.dependencies.get("WP01").unwrap(),
            &Vec::<String>::new()
        );
        assert_eq!(
            graph.dependencies.get("WP02").unwrap(),
            &vec!["WP01".to_string()]
        );
        assert_eq!(
            graph.dependencies.get("WP03").unwrap(),
            &vec!["WP01".to_string(), "WP02".to_string()]
        );
    }

    #[test]
    fn test_deps_satisfied_no_deps() {
        let wps = vec![create_test_wp("WP01", vec![])];
        let graph = DependencyGraph::new(&wps);

        let completed = HashSet::new();
        assert!(graph.deps_satisfied("WP01", &completed));
    }

    #[test]
    fn test_deps_satisfied_with_deps() {
        let wps = vec![
            create_test_wp("WP01", vec![]),
            create_test_wp("WP02", vec!["WP01"]),
        ];
        let graph = DependencyGraph::new(&wps);

        let mut completed = HashSet::new();
        assert!(!graph.deps_satisfied("WP02", &completed));

        completed.insert("WP01".to_string());
        assert!(graph.deps_satisfied("WP02", &completed));
    }

    #[test]
    fn test_get_dependents() {
        let wps = vec![
            create_test_wp("WP01", vec![]),
            create_test_wp("WP02", vec!["WP01"]),
            create_test_wp("WP03", vec!["WP01"]),
        ];
        let graph = DependencyGraph::new(&wps);

        let dependents = graph.get_dependents("WP01");
        assert_eq!(dependents.len(), 2);
        assert!(dependents.contains(&"WP02".to_string()));
        assert!(dependents.contains(&"WP03".to_string()));
    }

    #[test]
    fn test_get_dependents_none() {
        let wps = vec![
            create_test_wp("WP01", vec![]),
            create_test_wp("WP02", vec!["WP01"]),
        ];
        let graph = DependencyGraph::new(&wps);

        let dependents = graph.get_dependents("WP02");
        assert!(dependents.is_empty());
    }
}
