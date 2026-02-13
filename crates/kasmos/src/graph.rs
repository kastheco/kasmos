//! Dependency graph for work packages.
//!
//! This module provides a minimal dependency graph type that tracks work package
//! dependencies and allows efficient querying of dependency satisfaction and dependents.

use std::collections::{HashMap, HashSet, VecDeque};

/// A minimal dependency graph for work packages.
///
/// Tracks both forward dependencies (what a WP depends on) and reverse dependencies
/// (what depends on a WP) for efficient querying.
#[derive(Debug, Clone)]
pub struct DependencyGraph {
    /// Map from WP ID to its dependencies.
    pub(crate) dependencies: HashMap<String, Vec<String>>,

    /// Map from WP ID to its dependents (reverse dependencies).
    pub(crate) dependents: HashMap<String, Vec<String>>,
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
            dependents.entry(wp.id.clone()).or_default();

            // Build reverse dependencies
            for dep in &wp.dependencies {
                dependents
                    .entry(dep.clone())
                    .or_default()
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

    /// Topological sort using Kahn's algorithm.
    ///
    /// Returns work packages in dependency order (dependencies before dependents).
    /// Returns `SpecParserError::CircularDependency` if a cycle is detected.
    pub fn topological_sort(&self) -> crate::Result<Vec<String>> {
        // in_degree[X] = number of dependencies X has (edges pointing into X)
        let mut in_degree: HashMap<String, usize> = HashMap::new();
        for (wp_id, deps) in &self.dependencies {
            in_degree.insert(wp_id.clone(), deps.len());
        }

        // Start with nodes that have no dependencies
        let mut queue: VecDeque<String> = VecDeque::new();
        for (wp_id, &deg) in &in_degree {
            if deg == 0 {
                queue.push_back(wp_id.clone());
            }
        }

        let mut sorted = Vec::new();
        while let Some(wp_id) = queue.pop_front() {
            sorted.push(wp_id.clone());
            // For each WP that depends on wp_id, decrement their in-degree
            if let Some(dependents) = self.dependents.get(&wp_id) {
                for dep in dependents {
                    if let Some(count) = in_degree.get_mut(dep) {
                        *count -= 1;
                        if *count == 0 {
                            queue.push_back(dep.clone());
                        }
                    }
                }
            }
        }

        if sorted.len() != self.dependencies.len() {
            let unsorted: Vec<_> = self
                .dependencies
                .keys()
                .filter(|k| !sorted.contains(k))
                .cloned()
                .collect();
            return Err(crate::error::SpecParserError::CircularDependency {
                cycle: unsorted.join(" -> "),
            }
            .into());
        }

        Ok(sorted)
    }

    /// Compute wave assignments for all work packages.
    ///
    /// Wave 0 = no dependencies (roots). Wave N = all deps are in waves < N.
    /// Returns a Vec of waves, where each wave is a Vec of WP IDs.
    pub fn compute_waves(&self) -> crate::Result<Vec<Vec<String>>> {
        let sorted = self.topological_sort()?;
        if sorted.is_empty() {
            return Ok(Vec::new());
        }
        let mut wave_of: HashMap<String, usize> = HashMap::new();

        for wp_id in &sorted {
            let deps = self.dependencies.get(wp_id).cloned().unwrap_or_default();
            let wave = if deps.is_empty() {
                0
            } else {
                deps.iter()
                    .filter_map(|d| wave_of.get(d))
                    .max()
                    .map(|w| w + 1)
                    .unwrap_or(0)
            };
            wave_of.insert(wp_id.clone(), wave);
        }

        // Group by wave
        let max_wave = wave_of.values().max().copied().unwrap_or(0);
        let mut waves: Vec<Vec<String>> = vec![Vec::new(); max_wave + 1];
        for (wp_id, wave) in &wave_of {
            waves[*wave].push(wp_id.clone());
        }

        // Sort each wave for deterministic ordering
        for wave in &mut waves {
            wave.sort();
        }

        Ok(waves)
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

    #[test]
    fn test_topological_sort_empty() {
        let graph = DependencyGraph::new(&[]);
        let sorted = graph.topological_sort().unwrap();
        assert!(sorted.is_empty());
    }

    #[test]
    fn test_topological_sort_linear_chain() {
        let wps = vec![
            create_test_wp("WP01", vec![]),
            create_test_wp("WP02", vec!["WP01"]),
            create_test_wp("WP03", vec!["WP02"]),
        ];
        let graph = DependencyGraph::new(&wps);

        let sorted = graph.topological_sort().unwrap();
        let pos = |id: &str| sorted.iter().position(|s| s == id).unwrap();
        assert!(pos("WP01") < pos("WP02"));
        assert!(pos("WP02") < pos("WP03"));
    }

    #[test]
    fn test_topological_sort_diamond() {
        let wps = vec![
            create_test_wp("WP01", vec![]),
            create_test_wp("WP02", vec!["WP01"]),
            create_test_wp("WP03", vec!["WP01"]),
            create_test_wp("WP04", vec!["WP02", "WP03"]),
        ];
        let graph = DependencyGraph::new(&wps);

        let sorted = graph.topological_sort().unwrap();
        let pos = |id: &str| sorted.iter().position(|s| s == id).unwrap();
        assert!(pos("WP01") < pos("WP02"));
        assert!(pos("WP01") < pos("WP03"));
        assert!(pos("WP02") < pos("WP04"));
        assert!(pos("WP03") < pos("WP04"));
    }

    #[test]
    fn test_topological_sort_cycle_detection() {
        // Create a cycle: WP01 -> WP02 -> WP01
        let wps = vec![
            create_test_wp("WP01", vec!["WP02"]),
            create_test_wp("WP02", vec!["WP01"]),
        ];
        let graph = DependencyGraph::new(&wps);

        let result = graph.topological_sort();
        assert!(result.is_err());
    }

    #[test]
    fn test_compute_waves_empty() {
        let graph = DependencyGraph::new(&[]);
        let waves = graph.compute_waves().unwrap();
        assert!(waves.is_empty());
    }

    #[test]
    fn test_compute_waves_all_independent() {
        let wps = vec![
            create_test_wp("WP01", vec![]),
            create_test_wp("WP02", vec![]),
            create_test_wp("WP03", vec![]),
        ];
        let graph = DependencyGraph::new(&wps);

        let waves = graph.compute_waves().unwrap();
        assert_eq!(waves.len(), 1);
        assert_eq!(waves[0].len(), 3);
    }

    #[test]
    fn test_compute_waves_linear() {
        let wps = vec![
            create_test_wp("WP01", vec![]),
            create_test_wp("WP02", vec!["WP01"]),
            create_test_wp("WP03", vec!["WP02"]),
        ];
        let graph = DependencyGraph::new(&wps);

        let waves = graph.compute_waves().unwrap();
        assert_eq!(waves.len(), 3);
        assert_eq!(waves[0], vec!["WP01"]);
        assert_eq!(waves[1], vec!["WP02"]);
        assert_eq!(waves[2], vec!["WP03"]);
    }

    #[test]
    fn test_compute_waves_diamond() {
        let wps = vec![
            create_test_wp("WP01", vec![]),
            create_test_wp("WP02", vec!["WP01"]),
            create_test_wp("WP03", vec!["WP01"]),
            create_test_wp("WP04", vec!["WP02", "WP03"]),
        ];
        let graph = DependencyGraph::new(&wps);

        let waves = graph.compute_waves().unwrap();
        assert_eq!(waves.len(), 3);
        assert_eq!(waves[0], vec!["WP01"]);
        assert!(waves[1].contains(&"WP02".to_string()));
        assert!(waves[1].contains(&"WP03".to_string()));
        assert_eq!(waves[2], vec!["WP04"]);
    }
}
