import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { listInstances, listTasks, getInstanceCapture, pauseInstance, resumeInstance, restartInstance, killInstance, sendInstancePrompt } from "../api";
import { useAutoRefresh } from "../hooks/useAutoRefresh";
import { useProject } from "../hooks/useProject";
import { useToast } from "../hooks/useToast";
import TerminalPreview from "../components/TerminalPreview";
import AgentPreview from "../components/AgentPreview";
import ConfirmDialog from "../components/ConfirmDialog";
import InstanceActionsMenu from "../components/InstanceActionsMenu";
import type { InstanceEntry, InstanceAction, ScrollbackDepth, Status, TaskEntry } from "../types";
import {
  composerStateForInstance,
  shouldSubmitComposerKey,
  isAtBottom,
  previewLineLimit,
  captureErrorLabel,
  supportsStructuredPreview,
  usesTerminalPreview,
} from "./instanceInteractivity";
import styles from "./InstancesPage.module.css";
import {
  groupAgentsByTaskStatus,
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

/** Decides how a menu action should be routed. Pure so it can be tested
 *  without mounting the page — kill goes through a confirm dialog, everything
 *  else executes immediately. */
export type ActionRoute =
  | { type: "confirm-kill" }
  | { type: "immediate"; action: "pause" | "resume" | "restart" };

export function routeInstanceAction(action: InstanceAction): ActionRoute {
  if (action === "kill") return { type: "confirm-kill" };
  return { type: "immediate", action };
}

const DEPTH_STORAGE_KEY = "kasmos.instances.scrollbackDepth";
const DEPTH_OPTIONS: ScrollbackDepth[] = ["120", "1000", "full"];

function loadDepth(): ScrollbackDepth {
  try {
    const stored = localStorage.getItem(DEPTH_STORAGE_KEY);
    if (stored === "120" || stored === "1000" || stored === "full") return stored;
  } catch {
    // ignore
  }
  return "120";
}

function saveDepth(depth: ScrollbackDepth): void {
  try {
    localStorage.setItem(DEPTH_STORAGE_KEY, depth);
  } catch {
    // ignore
  }
}

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
  const [retiredOpen, setRetiredOpen] = useState(false);

  // -- capture state (page-local, not useAutoRefresh) --
  const [captureContent, setCaptureContent] = useState<string>("");
  const [captureError, setCaptureError] = useState<string | null>(null);
  const [captureLoading, setCaptureLoading] = useState(false);

  // -- follow mode & depth --
  const [isFollowing, setIsFollowing] = useState(true);
  const [depth, setDepth] = useState<ScrollbackDepth>(loadDepth);

  // -- composer --
  const [composerText, setComposerText] = useState("");
  const [composerError, setComposerError] = useState<string | null>(null);
  const [composerSending, setComposerSending] = useState(false);

  // -- refs --
  const previewRef = useRef<HTMLPreElement>(null);
  const pollTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const isFollowingRef = useRef(isFollowing);
  isFollowingRef.current = isFollowing;
  const depthRef = useRef(depth);
  depthRef.current = depth;

  // Reset selection and capture state when project changes.
  useEffect(() => {
    setSelectedTitle(null);
    setCaptureContent("");
    setCaptureError(null);
    setCaptureLoading(false);
    setIsFollowing(true);
  }, [project]);

  // Instance list — keep useAutoRefresh for the 2s list polling.
  const instances = useAutoRefresh<InstanceEntry[]>(
    () => (project ? listInstances(project) : Promise.resolve([])),
    [project],
    2000,
  );

  // Tasks list — refreshed less aggressively than instances since plan
  // lifecycle status changes on human timescales, not poll timescales.
  // Used only to map task_file → task status for stable grouping.
  const tasks = useAutoRefresh<TaskEntry[]>(
    () => (project ? listTasks(project) : Promise.resolve([])),
    [project],
    10000,
  );

  const taskStatusByFile = useMemo(() => {
    const m = new Map<string, Status>();
    for (const t of tasks.data ?? []) m.set(t.filename, t.status);
    return m;
  }, [tasks.data]);

  const groups = useMemo(
    () =>
      groupAgentsByTaskStatus(
        (instances.data ?? []).map((inst) =>
          toAgentCardModel(inst, inst.task_file ? taskStatusByFile.get(inst.task_file) : undefined),
        ),
        taskStatusByFile,
      ),
    [instances.data, taskStatusByFile],
  );
  const activeGroups = useMemo(
    () =>
      groups
        .map((group) => ({
          ...group,
          cards: group.cards.filter((card) => card.presentation !== "retired"),
        }))
        .filter((group) => group.cards.length > 0),
    [groups],
  );
  const retiredCards = useMemo(
    () => groups.flatMap((group) => group.cards.filter((card) => card.presentation === "retired")),
    [groups],
  );

  // O(1) lookup from card title → original InstanceEntry.
  const instanceMap = useMemo(
    () => new Map((instances.data ?? []).map((e) => [e.title, e])),
    [instances.data],
  );

  // Flat list of displayed cards in visual order — used for auto-selection
  // and for looking up the card that matches selectedTitle.
  const flatCards = useMemo(
    () => [...activeGroups.flatMap((g) => g.cards), ...retiredCards],
    [activeGroups, retiredCards],
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

  const selectedInstance =
    instances.data?.find((i) => i.title === selectedTitle) ?? null;
  const selectedCard =
    flatCards.find((c) => c.title === selectedTitle) ?? null;

  // Determine preview path: terminal (tmux) vs structured (AgentPreview).
  const isTerminalInstance = usesTerminalPreview(selectedInstance);

  // Capture poll logic.
  const doPoll = useCallback(async () => {
    if (!project || !selectedTitle) return;
    if (!isFollowingRef.current) return;

    try {
      const text = await getInstanceCapture(project, selectedTitle, {
        depth: depthRef.current,
      });
      setCaptureContent(text);
      setCaptureError(null);
      setCaptureLoading(false);

      // Snap to bottom if following.
      if (isFollowingRef.current && previewRef.current) {
        const el = previewRef.current;
        el.scrollTop = el.scrollHeight;
      }
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e);
      setCaptureError(msg);
      setCaptureLoading(false);
    }
  }, [project, selectedTitle]);

  // Schedule continuous polling when following. At "full" depth we do a
  // single fetch instead of polling — otherwise every second the client
  // would re-download the entire tmux history-limit buffer, which scales
  // bandwidth with scrollback size and can freeze the browser.
  useEffect(() => {
    if (pollTimerRef.current) {
      clearTimeout(pollTimerRef.current);
      pollTimerRef.current = null;
    }

    if (!isTerminalInstance || !isFollowing || !project || !selectedTitle) return;

    let cancelled = false;

    // One-shot fetch for "full" depth.
    if (depth === "full") {
      setCaptureLoading(true);
      void doPoll();
      return () => {
        cancelled = true;
      };
    }

    const schedule = () => {
      pollTimerRef.current = setTimeout(async () => {
        if (cancelled) return;
        await doPoll();
        if (!cancelled) schedule();
      }, 1000);
    };

    // Kick off first poll immediately.
    setCaptureLoading(true);
    void doPoll().then(() => {
      if (!cancelled) schedule();
    });

    return () => {
      cancelled = true;
      if (pollTimerRef.current) {
        clearTimeout(pollTimerRef.current);
        pollTimerRef.current = null;
      }
    };
  }, [isTerminalInstance, isFollowing, project, selectedTitle, depth, doPoll]);

  // Reset capture state when selected instance changes.
  useEffect(() => {
    setCaptureContent("");
    setCaptureError(null);
    setCaptureLoading(false);
    setIsFollowing(true);
    setComposerText("");
    setComposerError(null);
  }, [selectedTitle]);

  // Depth change: persist and trigger a re-poll on the next cycle.
  const handleDepthChange = (d: ScrollbackDepth) => {
    saveDepth(d);
    setDepth(d);
    // If already following, trigger an immediate poll on the new depth
    // by briefly toggling isFollowing so the effect re-fires.
    // Simpler: just call doPoll directly since depthRef is up to date.
    if (isFollowing && isTerminalInstance) {
      depthRef.current = d;
      void doPoll();
    }
  };

  // Scroll handler: detect if user scrolled away from bottom.
  const handlePreviewScroll = () => {
    const el = previewRef.current;
    if (!el) return;
    const atBottom = isAtBottom(el.scrollTop, el.clientHeight, el.scrollHeight);
    if (!atBottom && isFollowingRef.current) {
      setIsFollowing(false);
    }
  };

  // Jump-to-live: snap back to bottom and resume follow mode.
  const handleJumpToLive = () => {
    if (previewRef.current) {
      previewRef.current.scrollTop = previewRef.current.scrollHeight;
    }
    setIsFollowing(true);
  };

  // Composer submit.
  const handleSend = useCallback(async () => {
    if (!project || !selectedTitle || !composerText.trim()) return;
    setComposerSending(true);
    setComposerError(null);
    try {
      await sendInstancePrompt(project, selectedTitle, composerText);
      setComposerText("");
    } catch (e) {
      setComposerError(e instanceof Error ? e.message : String(e));
    } finally {
      setComposerSending(false);
    }
  }, [project, selectedTitle, composerText]);

  const composerState = composerStateForInstance(selectedInstance);
  const maxLines = previewLineLimit(depth);
  const captureErrLabel = captureErrorLabel(captureError);

  // ---- action handler --------------------------------------------------------

  const handleAction = useCallback(
    async (title: string, action: InstanceAction) => {
      if (!project) return;
      const route = routeInstanceAction(action);
      if (route.type === "confirm-kill") {
        setKillConfirmTitle(title);
        return;
      }
      setActionTitle(title);
      try {
        if (route.action === "pause") await pauseInstance(project, title);
        else if (route.action === "resume") await resumeInstance(project, title);
        else if (route.action === "restart") await restartInstance(project, title);
        toast.show(`'${title}' ${ACTION_PAST_TENSE[route.action]}`);
        await instances.refresh();
        // Refresh the capture panel immediately when the action targeted the
        // currently-selected row so the preview reflects the new state sooner
        // than the next 1s poll. Only applies to the terminal path — structured
        // preview rows poll via AgentPreview's own useAutoRefresh.
        if (selectedTitle === title && isTerminalInstance) {
          await doPoll();
        }
      } catch (err) {
        toast.show(String(err), { kind: "error" });
      } finally {
        setActionTitle(null);
      }
    },
    [project, instances, doPoll, selectedTitle, toast],
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
            {activeGroups.map((group) => (
              <section key={group.key} className={styles.group}>
                <h2 className={styles.groupHeader}>
                  <span className={`${styles.groupDot} ${styles[`dot_${group.key}`] ?? ""}`} />
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
            {retiredCards.length > 0 && (
              <section className={styles.group}>
                <h2 className={styles.groupHeader}>
                  <button
                    type="button"
                    className={styles.depthBtn}
                    aria-expanded={retiredOpen}
                    onClick={() => setRetiredOpen((open) => !open)}
                  >
                    {retiredOpen ? "▾" : "▸"}
                  </button>
                  <span>retired · {retiredCards.length}</span>
                </h2>
                {retiredOpen && (
                  <ul className={styles.list}>
                    {retiredCards.map((card) => {
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
                )}
              </section>
            )}
          </div>

          {/* right: preview + composer */}
          <div className={styles.preview}>
            {selectedCard ? (
              <>
                <div className={styles.previewHeader}>
                  <span className={styles.previewTitle}>{selectedCard.displayName}</span>

                  {/* depth presets and capture-error indicator: terminal path only */}
                  {isTerminalInstance && (
                    <div className={styles.previewControls}>
                      <div className={styles.depthPresets}>
                        {DEPTH_OPTIONS.map((d) => (
                          <button
                            key={d}
                            className={`${styles.depthBtn} ${d === depth ? styles.depthBtnActive : ""}`}
                            onClick={() => handleDepthChange(d)}
                          >
                            {d}
                          </button>
                        ))}
                      </div>

                      {captureErrLabel ? (
                        <span className={styles.captureError}>preview unavailable</span>
                      ) : null}
                    </div>
                  )}
                </div>

                {/* structured preview (daemon-managed SDK instances) */}
                {supportsStructuredPreview(selectedInstance) ? (
                  <AgentPreview
                    project={project!}
                    title={selectedTitle!}
                    onFollowStateChange={setIsFollowing}
                    onError={setCaptureError}
                  />
                ) : isTerminalInstance ? (
                  /* terminal (tmux) path */
                  captureErrLabel ? (
                    <p className={styles.captureEmpty}>{captureErrLabel}</p>
                  ) : (
                    <div className={styles.previewWrapper}>
                      <TerminalPreview
                        ref={previewRef}
                        content={captureContent}
                        maxLines={maxLines}
                        emptyLabel={captureLoading ? "loading…" : "waiting for output…"}
                        onScroll={handlePreviewScroll}
                      />
                      {!isFollowing && (
                        <button
                          className={styles.jumpToLive}
                          onClick={handleJumpToLive}
                        >
                          jump to live
                        </button>
                      )}
                    </div>
                  )
                ) : (
                  /* standalone SDK or unknown mode — no preview available */
                  <p className={styles.captureEmpty}>
                    preview not available for this instance
                  </p>
                )}

                {/* shared composer — works for both terminal and structured paths */}
                <div className={styles.composer}>
                  <textarea
                    className={styles.composerInput}
                    placeholder={
                      composerState.disabled
                        ? (composerState.reason ?? "unavailable")
                        : "send a message to this agent…"
                    }
                    value={composerText}
                    disabled={composerState.disabled || composerSending}
                    onChange={(e) => setComposerText(e.target.value)}
                    onKeyDown={(e) => {
                      if (shouldSubmitComposerKey(e)) {
                        e.preventDefault();
                        void handleSend();
                      }
                    }}
                    rows={3}
                  />
                  {composerError && (
                    <p className={styles.composerError}>{composerError}</p>
                  )}
                  <button
                    className={styles.composerSendBtn}
                    disabled={
                      composerState.disabled ||
                      composerSending ||
                      !composerText.trim()
                    }
                    onClick={() => void handleSend()}
                  >
                    {composerSending ? "sending…" : "send"}
                  </button>
                </div>
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
