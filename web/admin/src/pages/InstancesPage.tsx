import { useEffect, useState } from "react";
import { listInstances, getInstanceCapture } from "../api";
import { useAutoRefresh } from "../hooks/useAutoRefresh";
import { useProject } from "../hooks/useProject";
import StatusBadge from "../components/StatusBadge";
import TerminalPreview from "../components/TerminalPreview";
import type { InstanceEntry } from "../types";
import styles from "./InstancesPage.module.css";

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

export default function InstancesPage() {
  const { project } = useProject();
  const [selectedTitle, setSelectedTitle] = useState<string | null>(null);

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

  // Auto-select the first instance on first successful load.
  useEffect(() => {
    if (!instances.data) return;
    if (selectedTitle === null && instances.data.length > 0) {
      setSelectedTitle(instances.data[0].title);
      return;
    }
    // If the selected instance disappeared, pick the first remaining one.
    if (
      selectedTitle !== null &&
      !instances.data.find((i) => i.title === selectedTitle)
    ) {
      setSelectedTitle(instances.data.length > 0 ? instances.data[0].title : null);
    }
  }, [instances.data, selectedTitle]);

  const selectedInstance = instances.data?.find((i) => i.title === selectedTitle) ?? null;

  const captureContent = (() => {
    if (!selectedTitle) return "";
    if (capture.error) return "";
    return capture.data ?? "";
  })();

  return (
    <div className={styles.page}>
      <h1 className={styles.heading}>instances</h1>

      {instances.loading && !instances.data && (
        <p className={styles.empty}>loading…</p>
      )}

      {instances.error && !instances.data && (
        <p className={styles.errorMsg}>{instances.error}</p>
      )}

      {instances.data && instances.data.length === 0 && (
        <p className={styles.empty}>no instances found for this project</p>
      )}

      {instances.data && instances.data.length > 0 && (
        <div className={styles.split}>
          {/* left: instance list */}
          <ul className={styles.list}>
            {instances.data.map((inst) => (
              <li
                key={inst.title}
                className={`${styles.row} ${inst.title === selectedTitle ? styles.selected : ""}`}
                onClick={() => setSelectedTitle(inst.title)}
              >
                <div className={styles.rowHeader}>
                  <span className={styles.title}>{inst.title}</span>
                  <StatusBadge status={inst.status} />
                </div>
                {inst.task_file && (
                  <div className={styles.meta}>
                    <span className={styles.metaLabel}>task</span>
                    <span className={styles.metaValue}>{inst.task_file}</span>
                  </div>
                )}
                {(inst.wave_number != null || inst.task_number != null) && (
                  <div className={styles.meta}>
                    {inst.wave_number != null && (
                      <>
                        <span className={styles.metaLabel}>wave</span>
                        <span className={styles.metaValue}>{inst.wave_number}</span>
                      </>
                    )}
                    {inst.task_number != null && (
                      <>
                        <span className={styles.metaLabel}>task#</span>
                        <span className={styles.metaValue}>{inst.task_number}</span>
                      </>
                    )}
                  </div>
                )}
                {inst.branch && (
                  <div className={styles.meta}>
                    <span className={styles.metaLabel}>branch</span>
                    <span className={styles.metaValue}>{inst.branch}</span>
                  </div>
                )}
                {inst.updated_at && (
                  <div className={styles.meta}>
                    <span className={styles.metaLabel}>updated</span>
                    <span className={styles.metaValue}>{formatTime(inst.updated_at)}</span>
                  </div>
                )}
              </li>
            ))}
          </ul>

          {/* right: terminal preview */}
          <div className={styles.preview}>
            {selectedInstance ? (
              <>
                <div className={styles.previewHeader}>
                  <span className={styles.previewTitle}>{selectedInstance.title}</span>
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
              <p className={styles.empty}>select an instance to view its output</p>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
