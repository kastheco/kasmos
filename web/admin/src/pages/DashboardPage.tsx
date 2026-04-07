import { useMemo } from "react";
import { useLocation } from "react-router";
import { listTasks, listAuditEvents, resolveProjectName } from "../api";
import { useAutoRefresh } from "../hooks/useAutoRefresh";
import LastUpdated from "../components/LastUpdated";
import Skeleton from "../components/Skeleton";
import type { AuditEvent, Status, TaskEntry } from "../types";
import styles from "./DashboardPage.module.css";

type DashboardData = { tasks: TaskEntry[]; events: AuditEvent[] };

const STATUS_ORDER: Status[] = [
  "ready",
  "planning",
  "implementing",
  "reviewing",
  "done",
  "cancelled",
];

const STATUS_BORDER_CLASSES: Record<Status, string> = {
  ready: styles.ready,
  planning: styles.planning,
  implementing: styles.implementing,
  reviewing: styles.reviewing,
  done: styles.done,
  cancelled: styles.cancelled,
};

function countByStatus(tasks: TaskEntry[]): Record<Status, number> {
  const counts: Record<Status, number> = {
    ready: 0,
    planning: 0,
    implementing: 0,
    reviewing: 0,
    done: 0,
    cancelled: 0,
  };
  for (const task of tasks) {
    if (task.status in counts) {
      counts[task.status]++;
    }
  }
  return counts;
}

function formatRelativeTime(timestamp: string): string {
  const date = new Date(timestamp);
  if (isNaN(date.getTime())) return "just now";

  const rtf = new Intl.RelativeTimeFormat("en", { numeric: "auto" });
  const diffMs = date.getTime() - Date.now();
  const diffSec = Math.round(diffMs / 1000);

  if (Math.abs(diffSec) < 60) return rtf.format(diffSec, "second");
  const diffMin = Math.round(diffMs / 60000);
  if (Math.abs(diffMin) < 60) return rtf.format(diffMin, "minute");
  const diffHr = Math.round(diffMs / 3600000);
  if (Math.abs(diffHr) < 24) return rtf.format(diffHr, "hour");
  const diffDay = Math.round(diffMs / 86400000);
  return rtf.format(diffDay, "day");
}

export default function DashboardPage() {
  const { search } = useLocation();
  const project = useMemo(() => resolveProjectName(search), [search]);

  const { data, loading, error, lastUpdatedAt, isRefreshing } =
    useAutoRefresh<DashboardData>(
      async () => {
        const [tasks, allEvents] = await Promise.all([
          listTasks(project),
          listAuditEvents(project),
        ]);
        return { tasks, events: allEvents.slice(0, 20) };
      },
      [project],
    );

  const tasks = data?.tasks ?? [];
  const events = data?.events ?? [];
  const counts = useMemo(() => countByStatus(tasks), [tasks]);

  return (
    <div className={styles.page}>
      <div className={styles.titleRow}>
        <h1 className={styles.pageTitle}>dashboard</h1>
        <LastUpdated timestamp={lastUpdatedAt} isRefreshing={isRefreshing} />
      </div>

      {error && <div className={styles.error}>{error}</div>}

      {loading ? (
        <>
          <h2 className={styles.sectionTitle}>task status</h2>
          <div className={styles.cardsGrid}>
            {Array.from({ length: 6 }, (_, i) => (
              <Skeleton key={i} variant="card" />
            ))}
          </div>
          <h2 className={styles.sectionTitle}>recent activity</h2>
          {Array.from({ length: 5 }, (_, i) => (
            <Skeleton key={i} variant="row" />
          ))}
        </>
      ) : (
        <>
          <h2 className={styles.sectionTitle}>task status</h2>
          <div className={styles.cardsGrid}>
            {STATUS_ORDER.map((status) => (
              <div
                key={status}
                className={`${styles.statusCard} ${STATUS_BORDER_CLASSES[status]}`}
              >
                <div className={styles.statusCardLabel}>{status}</div>
                <div className={styles.statusCardCount}>{counts[status]}</div>
              </div>
            ))}
          </div>

          <div className={styles.activitySection}>
            <h2 className={styles.sectionTitle}>recent activity</h2>
            {events.length === 0 ? (
              <div className={styles.emptyActivity}>no recent activity</div>
            ) : (
              <div className={styles.activityList}>
                {events.map((event) => (
                  <div key={event.id} className={styles.activityItem}>
                    <span className={styles.activityKind}>{event.kind}</span>
                    <span className={styles.activityMessage}>
                      {event.message || event.detail || "—"}
                    </span>
                    <span className={styles.activityTime}>
                      {formatRelativeTime(event.timestamp)}
                    </span>
                  </div>
                ))}
              </div>
            )}
          </div>
        </>
      )}
    </div>
  );
}
