import { useCallback, useEffect, useMemo, useState } from "react";
import { listInstances, getInstanceCapture, pauseInstance, resumeInstance, restartInstance, killInstance } from "../api";
import { useAutoRefresh } from "../hooks/useAutoRefresh";
import { useProject } from "../hooks/useProject";
import { useToast } from "../hooks/useToast";
import TerminalPreview from "../components/TerminalPreview";
import ConfirmDialog from "../components/ConfirmDialog";
import InstanceActionsMenu from "../components/InstanceActionsMenu";
import type { InstanceEntry, InstanceAction } from "../types";
import styles from "./InstancesPage.module.css";
import {
  groupAgentsByStatus,
  toAgentCardModel,
  type AgentCardModel,
  type AgentPill,
} from "./agentCardModel";

const ACTION_PAST_TENSE: Record<InstanceAction, string> = {
  pause: "paused",
  resume: "resumed",
  restart: "restarted",
  kill: "killed",
};

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
  instance: InstanceEntry;
  selected: boolean;
  actionBusy: boolean;
  onSelect: () => void;
  onAction: (action: InstanceAction) => void;
}

function AgentCard({ card, instance, selected, actionBusy, onSelect, onAction }: AgentCardProps) {
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
        <InstanceActionsMenu
          instance={instance}
          busy={actionBusy}
          onAction={onAction}
        />
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
  const toast = useToast();
  const [selectedTitle, setSelectedTitle] = useState<string | null>(null);
  const [actionTitle, setActionTitle] = useState<string | null>(null);
  const [killConfirmTitle, setKillConfirmTitle] = useState<string | null>(null);

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

  // O(1) lookup from card title → original InstanceEntry.
  const instanceMap = useMemo(
    () => new Map((instances.data ?? []).map((e) => [e.title, e])),
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

  // ---- action handler --------------------------------------------------------

  const handleAction = useCallback(
    async (title: string, action: InstanceAction) => {
      if (!project) return;
      if (action === "kill") {
        // Kill requires confirmation — defer to the dialog.
        setKillConfirmTitle(title);
        return;
      }
      setActionTitle(title);
      try {
        if (action === "pause") await pauseInstance(project, title);
        else if (action === "resume") await resumeInstance(project, title);
        else if (action === "restart") await restartInstance(project, title);
        toast.show(`'${title}' ${ACTION_PAST_TENSE[action]}`);
        await instances.refresh();
        // Refresh the capture panel immediately when the action targeted the
        // currently-selected row so the preview reflects the new state sooner
        // than the next 1s poll.
        if (selectedTitle === title) {
          await capture.refresh();
        }
      } catch (err) {
        toast.show(String(err), { kind: "error" });
      } finally {
        setActionTitle(null);
      }
    },
    [project, instances, capture, selectedTitle, toast],
  );

  // Kill: confirmed path — keep killConfirmTitle set while the request is
  // in flight so the dialog stays visible in its busy state.
  const handleKillConfirm = useCallback(async () => {
    if (!killConfirmTitle || !project) return;
    const title = killConfirmTitle;
    setActionTitle(title);
    try {
      await killInstance(project, title);
      toast.show(`'${title}' ${ACTION_PAST_TENSE.kill}`);
      setKillConfirmTitle(null);
      await instances.refresh();
      // Selection reconciliation effect handles the now-missing row.
    } catch (err) {
      toast.show(String(err), { kind: "error" });
      setKillConfirmTitle(null);
    } finally {
      setActionTitle(null);
    }
  }, [killConfirmTitle, project, instances, toast]);

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
                  {group.cards.map((card) => {
                    const instance = instanceMap.get(card.title);
                    if (!instance) return null;
                    return (
                      <AgentCard
                        key={card.title}
                        card={card}
                        instance={instance}
                        selected={card.title === selectedTitle}
                        actionBusy={actionTitle === card.title}
                        onSelect={() => setSelectedTitle(card.title)}
                        onAction={(action) => void handleAction(card.title, action)}
                      />
                    );
                  })}
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

      {/* Kill confirmation dialog */}
      <ConfirmDialog
        open={killConfirmTitle !== null}
        title="kill instance"
        message={`kill '${killConfirmTitle ?? ""}'? this will terminate the agent session.`}
        confirmLabel="kill"
        destructive
        busy={actionTitle === killConfirmTitle && killConfirmTitle !== null}
        onConfirm={() => void handleKillConfirm()}
        onCancel={() => setKillConfirmTitle(null)}
      />
    </div>
  );
}
