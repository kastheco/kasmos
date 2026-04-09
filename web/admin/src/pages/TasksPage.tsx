import { useState } from "react";
import { Link } from "react-router";
import StatusBadge from "../components/StatusBadge";
import LastUpdated from "../components/LastUpdated";
import Skeleton from "../components/Skeleton";
import { listTasks } from "../api";
import { useAutoRefresh } from "../hooks/useAutoRefresh";
import { useProject } from "../hooks/useProject";
import type { Status, TaskEntry } from "../types";
import styles from "./TasksPage.module.css";

type TaskFilter = "all" | Status;
const FILTERS: TaskFilter[] = [
  "all",
  "ready",
  "planning",
  "implementing",
  "reviewing",
  "done",
  "cancelled",
];

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

  const [statusFilter, setStatusFilter] = useState<TaskFilter>("all");

  const { data, loading, error, lastUpdatedAt, isRefreshing } =
    useAutoRefresh<TaskEntry[]>(
      async () => {
        const data = await listTasks(project);
        return [...data].sort(
          (a, b) =>
            new Date(b.created_at ?? 0).getTime() -
            new Date(a.created_at ?? 0).getTime(),
        );
      },
      [project],
    );

  const allTasks = data ?? [];
  const tasks =
    statusFilter === "all"
      ? allTasks
      : allTasks.filter((t) => t.status === statusFilter);

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
      </div>

      {error && <div className={styles.error}>{error}</div>}

      {!loading && !error && tasks.length === 0 && (
        <div className={styles.empty}>no tasks found</div>
      )}

      {(loading || tasks.length > 0) && (
        <div className={styles.tableWrapper}>
          <table className={styles.table}>
            <thead>
              <tr>
                <th>status</th>
                <th>filename</th>
                <th>goal</th>
                <th>topic</th>
                <th>branch</th>
                <th>created</th>
              </tr>
            </thead>
            <tbody>
              {loading
                ? Array.from({ length: 5 }, (_, i) => (
                    <tr key={i}>
                      <td colSpan={6}>
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
                    </tr>
                  ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
