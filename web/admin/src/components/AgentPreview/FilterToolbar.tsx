import styles from "./FilterToolbar.module.css";

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export interface AgentPreviewFilters {
  hideTools: boolean;
  hideToolResults: boolean;
  hideThinking: boolean;
  hideSystem: boolean;
}

export const FILTER_STORAGE_KEY = "kasmos.agentPreview.filters";

export function loadFilters(): AgentPreviewFilters {
  try {
    const raw = localStorage.getItem(FILTER_STORAGE_KEY);
    if (raw) {
      const parsed = JSON.parse(raw) as Partial<AgentPreviewFilters>;
      return {
        hideTools: parsed.hideTools ?? false,
        hideToolResults: parsed.hideToolResults ?? false,
        hideThinking: parsed.hideThinking ?? false,
        hideSystem: parsed.hideSystem ?? false,
      };
    }
  } catch {
    // ignore parse errors
  }
  return { hideTools: false, hideToolResults: false, hideThinking: false, hideSystem: false };
}

export function saveFilters(filters: AgentPreviewFilters): void {
  try {
    localStorage.setItem(FILTER_STORAGE_KEY, JSON.stringify(filters));
  } catch {
    // ignore storage errors (e.g. private browsing quota)
  }
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

interface FilterToolbarProps {
  filters: AgentPreviewFilters;
  onChange: (filters: AgentPreviewFilters) => void;
}

/**
 * Thin strip with filter toggles for thinking, tools, tool results, and system rows.
 * State is owned by the parent and persisted to localStorage.
 * Permission rows are never affected by these filters.
 */
export function FilterToolbar({ filters, onChange }: FilterToolbarProps) {
  function toggle(key: keyof AgentPreviewFilters) {
    onChange({ ...filters, [key]: !filters[key] });
  }

  return (
    <div className={styles.toolbar} data-testid="filter-toolbar">
      <button
        className={`${styles.toggle} ${filters.hideThinking ? styles.active : ""}`}
        onClick={() => toggle("hideThinking")}
        aria-pressed={filters.hideThinking}
      >
        hide thinking
      </button>
      <button
        className={`${styles.toggle} ${filters.hideTools ? styles.active : ""}`}
        onClick={() => toggle("hideTools")}
        aria-pressed={filters.hideTools}
      >
        hide tools
      </button>
      <button
        className={`${styles.toggle} ${filters.hideToolResults ? styles.active : ""}`}
        onClick={() => toggle("hideToolResults")}
        aria-pressed={filters.hideToolResults}
      >
        hide tool results
      </button>
      <button
        className={`${styles.toggle} ${filters.hideSystem ? styles.active : ""}`}
        onClick={() => toggle("hideSystem")}
        aria-pressed={filters.hideSystem}
      >
        hide system
      </button>
    </div>
  );
}
