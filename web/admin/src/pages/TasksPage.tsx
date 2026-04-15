import { useEffect, useMemo, useRef, useState } from "react";
import { Link } from "react-router";
import StatusBadge from "../components/StatusBadge";
import LastUpdated from "../components/LastUpdated";
import Skeleton from "../components/Skeleton";
import TaskActionsMenu from "../components/TaskActionsMenu";
import NewTaskDialog, {
  type NewTaskDialogResult,
} from "../components/NewTaskDialog";
import { listTasks, listTopics } from "../api";
import { useAutoRefresh } from "../hooks/useAutoRefresh";
import { useProject } from "../hooks/useProject";
import { useToast } from "../hooks/useToast";
import type { Status, TaskEntry, TopicEntry } from "../types";
import styles from "./TasksPage.module.css";

type TaskFilter = "all" | Status;
const FILTERS: TaskFilter[] = [
  "all",
  "ready",
  "planning",
  "implementing",
  "reviewing",
  "verifying",
  "done",
  "cancelled",
];

type SortKey = "status" | "filename" | "goal" | "topic" | "branch" | "created";
type SortDir = "asc" | "desc";

const STATUS_ORDER: Record<Status, number> = {
  ready: 0,
  planning: 1,
  implementing: 2,
  reviewing: 3,
  verifying: 4,
  done: 5,
  cancelled: 6,
};

const DEFAULT_DIR: Record<SortKey, SortDir> = {
  status: "asc",
  filename: "asc",
  goal: "asc",
  topic: "asc",
  branch: "asc",
  created: "desc",
};

function sortValue(t: TaskEntry, key: SortKey): string | number | null {
  switch (key) {
    case "status":
      return STATUS_ORDER[t.status] ?? 999;
    case "filename":
      return t.filename ?? "";
    case "goal":
      return t.goal ?? "";
    case "topic":
      return t.topic ?? "";
    case "branch":
      return t.branch ?? "";
    case "created": {
      if (!t.created_at) return null;
      const ts = new Date(t.created_at).getTime();
      return isNaN(ts) ? null : ts;
    }
  }
}

function isEmpty(v: string | number | null): boolean {
  return v === null || v === "";
}

function truncate(text: string, max = 80): string {
  if (!text) return "";
  if (text.length <= max) return text;
  return text.slice(0, max) + "…";
}

function formatDate(value?: string): string {
  if (!value) return "—";
  const d = new Date(value);
  if (isNaN(d.getTime())) return "—";
  if (d.getFullYear() <= 1) return "—";
  return d.toISOString().slice(0, 10);
}

export default function TasksPage() {
  const { project, projectSearch } = useProject();
  const { show: showToast } = useToast();

  const [statusFilter, setStatusFilter] = useState<TaskFilter>("all");
  const [sortKey, setSortKey] = useState<SortKey>("created");
  const [sortDir, setSortDir] = useState<SortDir>("desc");
  const [dialogOpen, setDialogOpen] = useState(false);

  const newTaskButtonRef = useRef<HTMLButtonElement>(null);

  const { data, loading, error, lastUpdatedAt, isRefreshing, refresh: refreshTasks } =
    useAutoRefresh<TaskEntry[]>(
      async () => {
        if (!project) return [];
        return await listTasks(project);
      },
      [project],
    );

  const {
    data: topicsData,
    loading: topicsLoading,
    error: topicsError,
    refresh: refreshTopics,
  } = useAutoRefresh<TopicEntry[]>(
    async () => {
      if (!project) return [];
      return await listTopics(project);
    },
    [project],
  );

  // Topics are authoritative once the hook has returned non-null data at least
  // once. useAutoRefresh only leaves data=null while the first fetch is still
  // pending or failed outright; a successful load populates data and keeps it
  // even through subsequent background-refresh errors. Gating on data !== null
  // is therefore the cheapest correct "topics query is authoritative" check.
  const topicsReady = topicsData !== null;
  const topicsInitialFailure = !topicsReady && !topicsLoading && topicsError != null;
  const canOpenDialog = project !== "" && topicsReady;

  // Close the dialog when the project changes so a partially-complete create
  // flow cannot leak across projects.
  useEffect(() => {
    setDialogOpen(false);
  }, [project]);

  // If topics become non-authoritative for any reason (project switch while the
  // dialog is open, etc.), collapse the dialog instead of letting it run with a
  // stale or empty view. The backend stores topics with a case-sensitive key,
  // so submitting against a non-authoritative view risks forking identities.
  useEffect(() => {
    if (!topicsReady) setDialogOpen(false);
  }, [topicsReady]);

  function toggleSort(key: SortKey) {
    if (key === sortKey) {
      setSortDir((d) => (d === "asc" ? "desc" : "asc"));
    } else {
      setSortKey(key);
      setSortDir(DEFAULT_DIR[key]);
    }
  }

  function handleCloseDialog() {
    setDialogOpen(false);
    requestAnimationFrame(() => newTaskButtonRef.current?.focus());
  }

  async function handleCreated(result: NewTaskDialogResult) {
    await Promise.all([refreshTasks(), refreshTopics()]);
    setDialogOpen(false);

    const filename = result.task.filename;
    const { plannerRequested, warning } = result;

    if (warning?.stage === "content") {
      showToast(
        `task '${filename}' created; content save failed: ${warning.error.message}`,
        { kind: "error" },
      );
    } else if (warning?.stage === "plan_start") {
      showToast(
        `task '${filename}' created; planner queue failed: ${warning.error.message}`,
        { kind: "error" },
      );
    } else if (plannerRequested) {
      showToast(`task '${filename}' created, planner queued`);
    } else {
      showToast(`task '${filename}' created (ready)`);
    }
  }

  const allTasks = data ?? [];
  const filtered =
    statusFilter === "all"
      ? allTasks
      : allTasks.filter((t) => t.status === statusFilter);

  const tasks = useMemo(() => {
    const copy = [...filtered];
    copy.sort((a, b) => {
      const av = sortValue(a, sortKey);
      const bv = sortValue(b, sortKey);
      // empties always sort to the bottom, regardless of direction
      if (isEmpty(av) && isEmpty(bv)) return 0;
      if (isEmpty(av)) return 1;
      if (isEmpty(bv)) return -1;
      let cmp: number;
      if (typeof av === "number" && typeof bv === "number") {
        cmp = av - bv;
      } else {
        cmp = String(av).localeCompare(String(bv));
      }
      return sortDir === "asc" ? cmp : -cmp;
    });
    return copy;
  }, [filtered, sortKey, sortDir]);

  const countLabel =
    statusFilter === "all"
      ? `${tasks.length} tasks`
      : `${tasks.length} ${statusFilter} tasks`;

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <h1 className={styles.title}>{loading ? "tasks" : countLabel}</h1>
        <LastUpdated timestamp={lastUpdatedAt} isRefreshing={isRefreshing} />
        <div className={styles.filters}>
          {FILTERS.map((f) => (
            <button
              key={f}
              className={`${styles.filterBtn} ${statusFilter === f ? styles.filterActive : ""}`}
              onClick={() => setStatusFilter(f)}
            >
              {f}
            </button>
          ))}
        </div>
        <button
          ref={newTaskButtonRef}
          className={styles.newTaskBtn}
          onClick={() => setDialogOpen(true)}
          disabled={!canOpenDialog}
          title={
            project === ""
              ? undefined
              : !topicsReady && topicsLoading
                ? "loading topics\u2026"
                : topicsInitialFailure
                  ? `failed to load topics: ${topicsError}`
                  : undefined
          }
        >
          new task
        </button>
      </div>

      {topicsInitialFailure && (
        <div
          className={styles.topicsLoadError}
          role="alert"
          data-testid="topics-load-error"
        >
          <span>failed to load topics: {topicsError}</span>
          <button
            type="button"
            className={styles.topicsRetryBtn}
            onClick={() => {
              void refreshTopics();
            }}
          >
            retry
          </button>
        </div>
      )}

      {error && <div className={styles.error}>{error}</div>}

      {!loading && !error && tasks.length === 0 && (
        <div className={styles.empty}>no tasks found</div>
      )}

      {(loading || tasks.length > 0) && (
        <div className={styles.tableWrapper}>
          <table className={styles.table}>
            <thead>
              <tr>
                <SortHeader
                  k="status"
                  label="status"
                  sortKey={sortKey}
                  sortDir={sortDir}
                  onToggle={toggleSort}
                />
                <SortHeader
                  k="filename"
                  label="filename"
                  sortKey={sortKey}
                  sortDir={sortDir}
                  onToggle={toggleSort}
                />
                <SortHeader
                  k="goal"
                  label="goal"
                  sortKey={sortKey}
                  sortDir={sortDir}
                  onToggle={toggleSort}
                />
                <SortHeader
                  k="topic"
                  label="topic"
                  sortKey={sortKey}
                  sortDir={sortDir}
                  onToggle={toggleSort}
                />
                <SortHeader
                  k="branch"
                  label="branch"
                  sortKey={sortKey}
                  sortDir={sortDir}
                  onToggle={toggleSort}
                />
                <SortHeader
                  k="created"
                  label="created"
                  sortKey={sortKey}
                  sortDir={sortDir}
                  onToggle={toggleSort}
                />
                <th className={styles.actionsHeader}></th>
              </tr>
            </thead>
            <tbody>
              {loading
                ? Array.from({ length: 5 }, (_, i) => (
                    <tr key={i}>
                      <td colSpan={7}>
                        <Skeleton variant="row" />
                      </td>
                    </tr>
                  ))
                : tasks.map((task) => (
                    <tr key={task.filename} className={styles.row}>
                      <td>
                        <StatusBadge status={task.status} />
                      </td>
                      <td>
                        <Link
                          to={{
                            pathname: `/tasks/${encodeURIComponent(task.filename)}`,
                            search: projectSearch,
                          }}
                          className={styles.taskLink}
                        >
                          {task.filename}
                        </Link>
                      </td>
                      <td className={styles.goalCell}>
                        {truncate(task.goal ?? "")}
                      </td>
                      <td>{task.topic || "—"}</td>
                      <td className={styles.branchCell}>
                        {task.branch || "—"}
                      </td>
                      <td>{formatDate(task.created_at)}</td>
                      <td className={styles.actionsCell}>
                        {project && (
                          <TaskActionsMenu
                            project={project}
                            task={task}
                            variant="kebab"
                            onChanged={refreshTasks}
                            onDeleted={() => {
                              void refreshTasks();
                            }}
                          />
                        )}
                      </td>
                    </tr>
                  ))}
            </tbody>
          </table>
        </div>
      )}

      {/*
       * Gate `open` on topicsReady so the dialog never receives a
       * `topicsData ?? []` fallback that looks identical to "no topics exist".
       * When topics are not authoritative, canOpenDialog is false and the
       * button is disabled; this render is defence-in-depth in case the
       * button state ever drifts from topicsReady.
       *
       * Forward topicsError only as a non-blocking *refresh* warning, and
       * only once topics are already authoritative. A failed initial load
       * is handled above (disabled button + page-level retry banner); the
       * dialog never opens in that state, so it has no business seeing the
       * initial error. This split is what lets a transient background
       * refresh failure coexist with an open dialog without blocking
       * creation against the cached authoritative topics snapshot.
       */}
      <NewTaskDialog
        open={dialogOpen && topicsReady}
        project={project}
        topics={topicsData ?? []}
        topicsLoading={topicsLoading}
        topicsRefreshError={topicsReady ? topicsError : null}
        onRetryTopics={refreshTopics}
        onClose={handleCloseDialog}
        onCreated={handleCreated}
      />
    </div>
  );
}

interface SortHeaderProps {
  k: SortKey;
  label: string;
  sortKey: SortKey;
  sortDir: SortDir;
  onToggle: (k: SortKey) => void;
}

function SortHeader({ k, label, sortKey, sortDir, onToggle }: SortHeaderProps) {
  const active = sortKey === k;
  const arrow = active ? (sortDir === "asc" ? "▲" : "▼") : "";
  return (
    <th>
      <button
        type="button"
        className={`${styles.sortBtn} ${active ? styles.sortActive : ""}`}
        onClick={() => onToggle(k)}
        aria-sort={
          active ? (sortDir === "asc" ? "ascending" : "descending") : "none"
        }
      >
        <span>{label}</span>
        <span className={styles.sortArrow} aria-hidden="true">
          {arrow}
        </span>
      </button>
    </th>
  );
}
