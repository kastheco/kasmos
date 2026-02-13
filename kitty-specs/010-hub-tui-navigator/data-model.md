# Data Model: Hub TUI Navigator

## Entities

### FeatureEntry

Represents a feature discovered in `kitty-specs/`. This is the primary data unit displayed in the hub's feature list.

| Field | Type | Source | Description |
|---|---|---|---|
| `number` | `String` | Directory name prefix (e.g., "010") | Feature number for sorting and display |
| `slug` | `String` | Directory name suffix (e.g., "hub-tui-navigator") | Feature slug for display |
| `full_slug` | `String` | Full directory name (e.g., "010-hub-tui-navigator") | Used for path construction and Zellij session naming |
| `spec_status` | `SpecStatus` | `spec.md` existence + non-empty check | Whether the feature has a specification |
| `plan_status` | `PlanStatus` | `plan.md` existence check | Whether the feature has a plan |
| `task_progress` | `TaskProgress` | `tasks/WPxx-*.md` scan + frontmatter lane parsing | WP completion state |
| `orchestration_status` | `OrchestrationStatus` | `.kasmos/run.lock` + Zellij session list | Whether orchestration is running |
| `feature_dir` | `PathBuf` | Absolute path to `kitty-specs/<full_slug>/` | Used for all file operations |

### FeatureDetail

Expanded view of a single feature. Lazily loaded when the operator drills into a feature.

| Field | Type | Source | Description |
|---|---|---|---|
| `feature` | `FeatureEntry` | Parent entry | The feature being detailed |
| `work_packages` | `Vec<WPSummary>` | `tasks/WPxx-*.md` frontmatter | Individual WP states |

### WPSummary

Summary of a single work package, parsed from WP frontmatter.

| Field | Type | Source | Description |
|---|---|---|---|
| `id` | `String` | Frontmatter `work_package_id` | e.g., "WP01" |
| `title` | `String` | Frontmatter `title` | WP title for display |
| `lane` | `String` | Frontmatter `lane` | planned / doing / for_review / done |
| `wave` | `usize` | Frontmatter `wave` | Wave assignment |
| `dependencies` | `Vec<String>` | Frontmatter `dependencies` | WP IDs this depends on |

### HubAction

Contextual action available for a feature. Derived from `FeatureEntry` state, not stored.

| Variant | Condition | Effect |
|---|---|---|
| `CreateSpec` | `spec_status == Empty` | Opens OpenCode pane with `/spec-kitty.specify` |
| `NewFeature` | User selects "New Feature" entry | Prompts for name, then opens OpenCode pane |
| `Clarify` | `spec_status == Present` AND `plan_status == Absent` | Opens OpenCode pane with `/spec-kitty.clarify` |
| `Plan` | `spec_status == Present` AND `plan_status == Absent` | Opens OpenCode pane with `/spec-kitty.plan` |
| `GenerateTasks` | `plan_status == Present` AND `task_progress == NoTasks` | Opens OpenCode pane with `/spec-kitty.tasks` |
| `StartContinuous` | `task_progress` has pending WPs AND `orchestration_status` is `None` or `Completed` | Launches `kasmos start <feature>` in new tab (continuous mode) |
| `StartWaveGated` | Same as StartContinuous | Launches `kasmos start <feature> --mode wave-gated` in new tab |
| `Attach` | `orchestration_status == Running` | Switches to existing Zellij tab |
| `ViewDetails` | Always available | Shows FeatureDetail view |

## Enums

### SpecStatus
```
Empty    — spec.md missing or zero-length
Present  — spec.md exists and non-empty
```

### PlanStatus
```
Absent   — plan.md does not exist
Present  — plan.md exists
```

### TaskProgress
```
NoTasks              — tasks/ directory missing or no WPxx-*.md files
InProgress { done: usize, total: usize }  — some WPs exist, not all done
Complete { total: usize }                  — all WPs have lane "done"
```

### OrchestrationStatus
```
None       — no lock file or dead PID, no Zellij session
Running    — live lock file PID AND Zellij session exists
Completed  — no live process but Zellij session exists (EXITED state)
```

### HubView
```
List                    — feature list (main view)
Detail { index: usize } — expanded feature detail
```

### InputMode
```
Normal                        — standard navigation
NewFeaturePrompt { input: String }  — typing a new feature name
ConfirmDialog { message: String, on_confirm: Box<HubAction> }  — confirmation modal (e.g., >6 WP warning)
```

### PaneDirection
```
Right  — open pane to the right (side-by-side with hub)
Down   — open pane below (stacking)
```

## State Transitions

### Feature Lifecycle (as seen by the hub)

```
[Empty Spec] ──CreateSpec──→ [Spec Present] ──Plan──→ [Plan Present]
                                    │                       │
                                    ├──Clarify──→ [Spec Present] (updated)
                                    │                       │
                                    └────────────────── GenerateTasks──→ [Tasks Ready]
                                                                            │
                                                            StartImplementation──→ [Running]
                                                                            │
                                                                        [Complete]
```

The hub does not drive these transitions — it detects them by re-scanning the filesystem. Transitions happen in OpenCode agent panes or orchestration sessions.

### Hub View Navigation

```
[List View] ──Enter (on feature)──→ [Detail View] ──Esc──→ [List View]
     │                                    │
     ├──'n' (new feature)──→ [NewFeaturePrompt] ──Enter──→ [List View] + agent pane
     │                              │
     │                          Esc──→ [List View]
     │
     ├──Enter (on action)──→ [action dispatched] (pane/tab opens)
     │
     └──Alt+q──→ [quit]
```

## Relationships

- `FeatureEntry` 1:1 `FeatureDetail` (detail is lazily expanded from entry)
- `FeatureDetail` 1:N `WPSummary` (a feature can have 0..N work packages)
- `FeatureEntry` → N `HubAction` (actions are derived from entry state, not stored)
- `HubAction` → 1 `PaneDirection` (each pane action has a direction: right for agents, tab for implementation)
