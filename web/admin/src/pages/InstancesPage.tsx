import { useEffect, useMemo, useState } from "react";
import { listInstances, getInstanceCapture } from "../api";
import { useAutoRefresh } from "../hooks/useAutoRefresh";
import { useProject } from "../hooks/useProject";
import TerminalPreview from "../components/TerminalPreview";
import type { InstanceEntry } from "../types";
import styles from "./InstancesPage.module.css";
import {
  groupAgentsByStatus,
  toAgentCardModel,
  type AgentCardModel,
  type AgentPill,
} from "./agentCardModel";

function formatTime(iso?: string): string {
  if (!iso) return "";
  try {
    return new Date(iso).toLocaleTimeString([], {
      hour: "2-digit",
      minute: "2-digit",
      second: "2-digit",
    });
  } catch {
    return iso;
  }
}

function toneClass(pill: AgentPill): string {
  switch (pill.tone) {
    case "wave":
      return styles.pillWave;
    case "task":
      return styles.pillTask;
    case "cycle":
      return styles.pillCycle;
    case "role":
      return styles.pillRole;
    default:
      return styles.pillDefault;
  }
}

interface AgentCardProps {
  card: AgentCardModel;
  selected: boolean;
  onSelect: () => void;
}

function AgentCard({ card, selected, onSelect }: AgentCardProps) {
  return (
    <li
      role="button"
      tabIndex={0}
      aria-pressed={selected}
      className={`${styles.row} ${selected ? styles.selected : ""}`}
      onClick={onSelect}
      onKeyDown={(e) => {
        if (e.key === "Enter" || e.key === " ") {
          e.preventDefault();
          onSelect();
        }
      }}
    >
      <div className={styles.rowHeader}>
        <span className={styles.title}>{card.displayName}</span>
      </div>
      {card.pills.length > 0 && (
        <div className={styles.pillRow}>
          {card.pills.map((p, i) => (
            <span key={i} className={`${styles.pill} ${toneClass(p)}`}>
              {p.label}
            </span>
          ))}
        </div>
      )}
      {card.branch && (
        <div className={styles.meta}>
          <span className={styles.metaLabel}>branch</span>
          <span className={styles.metaValue}>{card.branch}</span>
        </div>
      )}
      {card.updatedAt && (
        <div className={styles.meta}>
          <span className={styles.metaLabel}>updated</span>
          <span className={styles.metaValue}>{formatTime(card.updatedAt)}</span>
        </div>
      )}
    </li>
  );
}

export default function InstancesPage() {
  const { project } = useProject();
  const [selectedTitle, setSelectedTitle] = useState<string | null>(null);

  // Reset selection when the active project changes to avoid stale capture polls.
  useEffect(() => {
    setSelectedTitle(null);
  }, [project]);

  const instances = useAutoRefresh<InstanceEntry[]>(
    () => (project ? listInstances(project) : Promise.resolve([])),
    [project],
    2000,
  );

  const capture = useAutoRefresh<string>(
    () =>
      project && selectedTitle
        ? getInstanceCapture(project, selectedTitle, { start: "-120" })
        : Promise.resolve(""),
    [project, selectedTitle],
    1000,
  );

  const groups = useMemo(
    () => groupAgentsByStatus((instances.data ?? []).map(toAgentCardModel)),
    [instances.data],
  );

  // Flat list of displayed cards in visual order — used for auto-selection
  // and for looking up the card that matches selectedTitle.
  const flatCards = useMemo(
    () => groups.flatMap((g) => g.cards),
    [groups],
  );

  // Auto-select the first instance on first successful load; reassign when
  // the selected instance disappears from the list.
  useEffect(() => {
    if (!instances.data) return;
    if (selectedTitle === null && flatCards.length > 0) {
      setSelectedTitle(flatCards[0].title);
      return;
    }
    if (
      selectedTitle !== null &&
      !flatCards.find((c) => c.title === selectedTitle)
    ) {
      setSelectedTitle(flatCards.length > 0 ? flatCards[0].title : null);
    }
  }, [instances.data, selectedTitle, flatCards]);

  const selectedCard =
    flatCards.find((c) => c.title === selectedTitle) ?? null;

  const captureContent = (() => {
    if (!selectedTitle) return "";
    if (capture.error) return "";
    return capture.data ?? "";
  })();

  return (
    <div className={styles.page}>
      <h1 className={styles.heading}>agents</h1>

      {instances.loading && !instances.data && (
        <p className={styles.empty}>loading…</p>
      )}

      {instances.error && !instances.data && (
        <p className={styles.errorMsg}>{instances.error}</p>
      )}

      {instances.data && instances.data.length === 0 && (
        <p className={styles.empty}>no agents running for this project</p>
      )}

      {instances.data && instances.data.length > 0 && (
        <div className={styles.split}>
          {/* left: grouped agent list */}
          <div className={styles.listColumn}>
            {groups.map((group) => (
              <section key={group.status} className={styles.group}>
                <h2 className={styles.groupHeader}>
                  <span className={`${styles.groupDot} ${styles[`dot_${group.status}`]}`} />
                  <span>{group.label}</span>
                  <span className={styles.groupCount}>{group.cards.length}</span>
                </h2>
                <ul className={styles.list}>
                  {group.cards.map((card) => (
                    <AgentCard
                      key={card.title}
                      card={card}
                      selected={card.title === selectedTitle}
                      onSelect={() => setSelectedTitle(card.title)}
                    />
                  ))}
                </ul>
              </section>
            ))}
          </div>

          {/* right: terminal preview */}
          <div className={styles.preview}>
            {selectedCard ? (
              <>
                <div className={styles.previewHeader}>
                  <span className={styles.previewTitle}>{selectedCard.displayName}</span>
                  {capture.error ? (
                    <span className={styles.captureError}>preview unavailable</span>
                  ) : null}
                </div>
                {capture.error ? (
                  <p className={styles.captureEmpty}>
                    pane output is not available right now
                  </p>
                ) : (
                  <TerminalPreview
                    content={captureContent}
                    maxLines={80}
                    emptyLabel="waiting for output…"
                  />
                )}
              </>
            ) : (
              <p className={styles.empty}>select an agent to view its output</p>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
