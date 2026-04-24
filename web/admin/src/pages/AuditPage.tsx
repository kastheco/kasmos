import { useState, useEffect } from "react";
import { Link } from "react-router";
import { fetchAuditEvents, parseKillDetail } from "../api";
import { useAutoRefresh } from "../hooks/useAutoRefresh";
import { useProject } from "../hooks/useProject";
import LastUpdated from "../components/LastUpdated";
import Skeleton from "../components/Skeleton";
import type { AuditEvent } from "../types";
import styles from "./AuditPage.module.css";

const EVENT_KINDS = [
  "agent_spawned",
  "agent_finished",
  "agent_killed",
  "agent_paused",
  "agent_resumed",
  "agent_restarted",
  "plan_transition",
  "plan_created",
  "plan_merged",
  "plan_cancelled",
  "wave_started",
  "wave_completed",
  "wave_failed",
  "prompt_sent",
  "git_push",
  "pr_created",
  "permission_detected",
  "permission_answered",
  "fsm_error",
  "error",
  "session_started",
  "session_stopped",
] as const;

const LIMIT_OPTIONS = [25, 50, 100, 200, 500] as const;

function formatDateTime(value: string): string {
  if (!value) return "—";
  const d = new Date(value);
  if (isNaN(d.getTime())) return value;
  return d.toLocaleString();
}

function prettyDetail(detail: string): string {
  if (!detail) return "";
  try {
    return JSON.stringify(JSON.parse(detail), null, 2);
  } catch {
    return detail;
  }
}

function isWaveDecision(event: AuditDisplayEvent): boolean {
  if (event.kind !== "wave_failed" || !event.detail) return false;
  try {
    const detail = JSON.parse(event.detail) as { outcome?: unknown };
    return detail.outcome === "wave_decision";
  } catch {
    return false;
  }
}

type AuditDisplayEvent = AuditEvent & {
  groupedEvents?: AuditEvent[];
};

function killGroupKey(event: AuditEvent): string | null {
  const detail = parseKillDetail(event);
  return detail?.group_key ?? null;
}

function killCleanup(event: AuditEvent): boolean {
  return parseKillDetail(event)?.cleanup ?? false;
}

function coalesceAuditRows(events: AuditEvent[]): AuditDisplayEvent[] {
  const rows: AuditDisplayEvent[] = [];
  for (let i = 0; i < events.length; ) {
    const event = events[i];
    const groupKey = event.kind === "agent_killed" ? killGroupKey(event) : null;
    if (!groupKey) {
      rows.push(event);
      i += 1;
      continue;
    }

    const group = [event];
    let best = event;
    let j = i + 1;
    while (
      j < events.length &&
      events[j].kind === "agent_killed" &&
      killGroupKey(events[j]) === groupKey
    ) {
      group.push(events[j]);
      if (killCleanup(events[j]) && !killCleanup(best)) {
        best = events[j];
      }
      j += 1;
    }

    rows.push(group.length > 1 ? { ...best, groupedEvents: group } : best);
    i = j;
  }
  return rows;
}

function prettyEventDetail(event: AuditDisplayEvent): string {
  if (!event.groupedEvents || event.groupedEvents.length < 2) {
    return prettyDetail(event.detail);
  }
  return event.groupedEvents
    .map((grouped) => `event ${grouped.id}\n${prettyDetail(grouped.detail)}`)
    .join("\n\n");
}

type Tone = "lifecycle" | "plan" | "wave" | "ops" | "error";

const LIFECYCLE_KINDS = new Set([
  "agent_spawned",
  "agent_finished",
  "agent_killed",
  "agent_paused",
  "agent_resumed",
  "agent_restarted",
  "session_started",
  "session_stopped",
]);

const PLAN_KINDS = new Set([
  "plan_transition",
  "plan_created",
  "plan_merged",
  "plan_cancelled",
]);

const WAVE_KINDS = new Set(["wave_started", "wave_completed", "wave_failed"]);
const ERROR_KINDS = new Set(["fsm_error", "error"]);

function kindTone(kind: string): Tone {
  if (LIFECYCLE_KINDS.has(kind)) return "lifecycle";
  if (PLAN_KINDS.has(kind)) return "plan";
  if (WAVE_KINDS.has(kind)) return "wave";
  if (ERROR_KINDS.has(kind)) return "error";
  return "ops";
}

const TONE_CLASSES: Record<Tone, string> = {
  lifecycle: styles.toneLifecycle,
  plan: styles.tonePlan,
  wave: styles.toneWave,
  ops: styles.toneOps,
  error: styles.toneError,
};

function levelClass(level: string): string {
  const l = level || "info";
  if (l === "error") return styles.levelError;
  if (l === "warn") return styles.levelWarn;
  return styles.levelInfo;
}

export default function AuditPage() {
  const { project, projectSearch } = useProject();

  const [kind, setKind] = useState("");
  const [taskInput, setTaskInput] = useState("");
  const [taskFile, setTaskFile] = useState("");
  const [instanceInput, setInstanceInput] = useState("");
  const [instanceTitle, setInstanceTitle] = useState("");
  const [after, setAfter] = useState("");
  const [before, setBefore] = useState("");
  const [limit, setLimit] = useState(100);
  const [expandedRowId, setExpandedRowId] = useState<number | null>(null);

  // Debounce taskInput -> taskFile (300ms)
  useEffect(() => {
    const timer = setTimeout(() => setTaskFile(taskInput), 300);
    return () => clearTimeout(timer);
  }, [taskInput]);

  useEffect(() => {
    const timer = setTimeout(() => setInstanceTitle(instanceInput), 300);
    return () => clearTimeout(timer);
  }, [instanceInput]);

  const { data, loading, error, lastUpdatedAt, isRefreshing } =
    useAutoRefresh<AuditEvent[]>(
      async () => {
        if (!project) return [];
        return fetchAuditEvents(project, {
          kind: kind || undefined,
          task: taskFile || undefined,
          instance: instanceTitle || undefined,
          after: after || undefined,
          before: before || undefined,
          limit,
        });
      },
      [project, kind, taskFile, instanceTitle, after, before, limit],
    );

  const events = data ?? [];

  // Clear expandedRowId when the refreshed event list no longer contains it
  useEffect(() => {
    if (expandedRowId !== null && !events.some((e) => e.id === expandedRowId)) {
      setExpandedRowId(null);
    }
  }, [events, expandedRowId]);

  function toggleRow(event: AuditEvent) {
    if (!event.detail) return;
    setExpandedRowId((prev) => (prev === event.id ? null : event.id));
  }

  const displayEvents = coalesceAuditRows(events);
  const rows = displayEvents.flatMap((event) => {
    const isExpanded = expandedRowId === event.id;
    const hasDetail = Boolean(event.detail);
    const rowClasses = [
      styles.row,
      hasDetail ? styles.expandable : "",
      isExpanded ? styles.expanded : "",
    ]
      .filter(Boolean)
      .join(" ");

    const mainRow = (
      <tr
        key={event.id}
        className={rowClasses}
        onClick={() => toggleRow(event)}
        tabIndex={hasDetail ? 0 : undefined}
        role={hasDetail ? "button" : undefined}
        aria-expanded={hasDetail ? isExpanded : undefined}
        onKeyDown={
          hasDetail
            ? (e) => {
                if (e.key === "Enter" || e.key === " ") {
                  e.preventDefault();
                  toggleRow(event);
                }
              }
            : undefined
        }
      >
        <td className={styles.timestamp}>{formatDateTime(event.timestamp)}</td>
        <td>
            <span
              className={`${styles.badge} ${levelClass(
                isWaveDecision(event) && event.level === "error" ? "warn" : event.level,
              )}`}
            >
              {isWaveDecision(event) && event.level === "error"
                ? "warn"
                : event.level || "info"}
            </span>
        </td>
        <td>
          <span
            className={`${styles.kindBadge} ${TONE_CLASSES[kindTone(event.kind)]}`}
          >
            {event.kind}
          </span>
        </td>
        <td>
          {event.task_file ? (
            <Link
              to={{
                pathname: `/tasks/${encodeURIComponent(event.task_file)}`,
                search: projectSearch,
              }}
              className={styles.taskLink}
              onClick={(e) => e.stopPropagation()}
            >
              {event.task_file}
            </Link>
          ) : (
            <span className={styles.empty}>—</span>
          )}
        </td>
        <td className={styles.message}>{event.message}</td>
      </tr>
    );

    if (!isExpanded || !hasDetail) {
      return [mainRow];
    }

    const detailRow = (
      <tr key={`detail-${event.id}`} className={styles.detailRow}>
        <td colSpan={5}>
          <pre className={styles.detailPre}>{prettyEventDetail(event)}</pre>
        </td>
      </tr>
    );

    return [mainRow, detailRow];
  });

  return (
    <div className={styles.page}>
      <div className={styles.headingRow}>
        <h1 className={styles.heading}>audit log</h1>
        <LastUpdated timestamp={lastUpdatedAt} isRefreshing={isRefreshing} />
      </div>

      <div className={styles.filters}>
        <select
          className={styles.filterSelect}
          value={kind}
          onChange={(e) => setKind(e.target.value)}
        >
          <option value="">all kinds</option>
          {EVENT_KINDS.map((k) => (
            <option key={k} value={k}>
              {k}
            </option>
          ))}
        </select>

        <input
          className={styles.filterInput}
          type="text"
          placeholder="filter by task file..."
          value={taskInput}
          onChange={(e) => setTaskInput(e.target.value)}
        />

        <input
          className={styles.filterInput}
          type="text"
          placeholder="filter by instance..."
          value={instanceInput}
          onChange={(e) => setInstanceInput(e.target.value)}
        />

        <input
          className={styles.filterInput}
          type="text"
          aria-label="after"
          placeholder="after RFC3339..."
          value={after}
          onChange={(e) => setAfter(e.target.value)}
        />

        <input
          className={styles.filterInput}
          type="text"
          aria-label="before"
          placeholder="before RFC3339..."
          value={before}
          onChange={(e) => setBefore(e.target.value)}
        />

        <select
          className={styles.filterSelect}
          value={limit}
          onChange={(e) => setLimit(Number(e.target.value))}
        >
          {LIMIT_OPTIONS.map((n) => (
            <option key={n} value={n}>
              limit: {n}
            </option>
          ))}
        </select>
      </div>

      {error && <p className={styles.errorMsg}>{error}</p>}

      <div className={styles.tableWrapper}>
        {loading ? (
          <>
            <Skeleton variant="row" />
            {Array.from({ length: 5 }, (_, i) => (
              <Skeleton key={i} variant="row" />
            ))}
          </>
        ) : (
          <table className={styles.table}>
            <thead>
              <tr>
                <th>timestamp</th>
                <th>level</th>
                <th>kind</th>
                <th>task</th>
                <th>message</th>
              </tr>
            </thead>
            <tbody>
              {rows}
              {events.length === 0 && (
                <tr>
                  <td colSpan={5} className={styles.emptyCell}>
                    no events found
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        )}
      </div>
    </div>
  );
}
