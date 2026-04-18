import { useEffect, useRef, useState } from "react";
import { getInstancePresentation } from "../../api";
import { useAutoRefresh } from "../../hooks/useAutoRefresh";
import { isAtBottom } from "../../pages/instanceInteractivity";
import { FilterToolbar, loadFilters, saveFilters } from "./FilterToolbar";
import type { AgentPreviewFilters } from "./FilterToolbar";
import { TurnTimeline } from "./TurnTimeline";
import styles from "./AgentPreview.module.css";

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

interface AgentPreviewProps {
  project: string;
  title: string;
  onFollowStateChange?: (following: boolean) => void;
  onError?: (message: string | null) => void;
}

/**
 * Structured-preview entry point for daemon-managed SDK instances.
 * Owns its own 1-second polling via useAutoRefresh and its own scroll container.
 * Uses `isAtBottom` from instanceInteractivity to track follow mode.
 *
 * Layout: outer wrapper (flex-column) → FilterToolbar (fixed height) + inner
 * scroll container. This keeps the filter strip always visible while the
 * timeline scrolls independently.
 *
 * Do not add terminal depth controls here — they are tmux-only.
 */
export default function AgentPreview({
  project,
  title,
  onFollowStateChange,
  onError,
}: AgentPreviewProps) {
  const containerRef = useRef<HTMLDivElement>(null);

  // snapshotReceivedAt: browser timestamp (ms) when the last snapshot arrived.
  // Used to keep elapsed labels accurate between server capture time and render.
  const snapshotReceivedAt = useRef<number>(Date.now());

  const [isFollowing, setIsFollowing] = useState(true);
  const isFollowingRef = useRef(isFollowing);
  isFollowingRef.current = isFollowing;

  // ---------------------------------------------------------------------------
  // Filter state — persisted to localStorage["kasmos.agentPreview.filters"]
  // ---------------------------------------------------------------------------

  const [filters, setFilters] = useState<AgentPreviewFilters>(loadFilters);

  function handleFiltersChange(next: AgentPreviewFilters) {
    setFilters(next);
    saveFilters(next);
  }

  // 1-second poll via useAutoRefresh.
  const presentation = useAutoRefresh(
    () => getInstancePresentation(project, title),
    [project, title],
    1000,
  );

  // Record when each snapshot arrives.
  useEffect(() => {
    if (presentation.data) {
      snapshotReceivedAt.current = Date.now();
    }
  }, [presentation.data]);

  // Propagate error changes to parent.
  useEffect(() => {
    onError?.(presentation.error);
  }, [presentation.error, onError]);

  // Propagate follow-state changes to parent.
  useEffect(() => {
    onFollowStateChange?.(isFollowing);
  }, [isFollowing, onFollowStateChange]);

  // Follow-mode scroll: snap to the last prose row in the newest turn,
  // or fall back to scrollHeight when no prose rows exist yet.
  useEffect(() => {
    if (!isFollowing || !containerRef.current || !presentation.data) return;
    const container = containerRef.current;
    const proseRows = container.querySelectorAll<HTMLElement>('[data-kind="prose"]');
    if (proseRows.length > 0) {
      const lastProse = proseRows[proseRows.length - 1];
      // scrollIntoView is not available in all environments (e.g. jsdom).
      if (typeof lastProse.scrollIntoView === "function") {
        lastProse.scrollIntoView({ block: "nearest" });
      }
    } else {
      container.scrollTop = container.scrollHeight;
    }
  }, [presentation.data, isFollowing]);

  // Scroll handler: detect when user scrolls away from bottom.
  const handleScroll = () => {
    const el = containerRef.current;
    if (!el) return;
    const atBottom = isAtBottom(el.scrollTop, el.clientHeight, el.scrollHeight);
    if (!atBottom && isFollowingRef.current) {
      setIsFollowing(false);
    }
  };

  // ---------------------------------------------------------------------------
  // Render helpers
  // ---------------------------------------------------------------------------

  const data = presentation.data;
  const turns = data?.turns ?? null;

  const toolbar = (
    <FilterToolbar filters={filters} onChange={handleFiltersChange} />
  );

  // ---------------------------------------------------------------------------
  // Early-return states — wrapper always shown so layout is stable.
  // ---------------------------------------------------------------------------

  if (presentation.loading && !data) {
    return (
      <div className={styles.wrapper}>
        {toolbar}
        <div ref={containerRef} className={styles.container} onScroll={handleScroll}>
          <p className={styles.loading}>loading…</p>
        </div>
      </div>
    );
  }

  if (presentation.error && !data) {
    return (
      <div className={styles.wrapper}>
        {toolbar}
        <div ref={containerRef} className={styles.container} onScroll={handleScroll}>
          <p className={styles.empty}>{presentation.error}</p>
        </div>
      </div>
    );
  }

  if (data && !data.supported) {
    return (
      <div className={styles.wrapper}>
        {toolbar}
        <div ref={containerRef} className={styles.container} onScroll={handleScroll}>
          <p className={styles.empty}>
            structured preview is not supported for this instance
          </p>
        </div>
      </div>
    );
  }

  if (!turns || turns.length === 0) {
    return (
      <div className={styles.wrapper}>
        {toolbar}
        <div ref={containerRef} className={styles.container} onScroll={handleScroll}>
          <p className={styles.empty}>waiting for agent output…</p>
        </div>
      </div>
    );
  }

  return (
    <div className={styles.wrapper}>
      {toolbar}
      <div ref={containerRef} className={styles.container} onScroll={handleScroll}>
        <TurnTimeline
          turns={turns}
          capturedAt={data!.captured_at}
          snapshotReceivedAt={snapshotReceivedAt.current}
          project={project}
          title={title}
          filters={filters}
        />
      </div>
    </div>
  );
}
