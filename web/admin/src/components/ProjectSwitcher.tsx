import type { JSX } from "react";
import { useProject } from "../hooks/useProject";
import styles from "./ProjectSwitcher.module.css";

export default function ProjectSwitcher(): JSX.Element {
  const { project, projects, loading, setProject } = useProject();

  const disabled = loading || projects.length === 0;

  return (
    <div className={styles.switcher}>
      <span className={styles.label}>project</span>
      <select
        className={styles.select}
        value={project}
        disabled={disabled}
        onChange={(e) => setProject(e.target.value)}
        aria-label="select project"
      >
        {disabled ? (
          <option value="">{loading ? "loading…" : "no projects"}</option>
        ) : (
          projects.map((p) => (
            <option key={p} value={p}>
              {p}
            </option>
          ))
        )}
      </select>
    </div>
  );
}
