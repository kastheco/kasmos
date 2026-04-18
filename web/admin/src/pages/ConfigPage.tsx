import { useState, useEffect, useCallback } from "react";
import { useProject } from "../hooks/useProject";
import { useToast } from "../hooks/useToast";
import LastUpdated from "../components/LastUpdated";
import ConfirmDialog from "../components/ConfirmDialog";
import {
  getProjectConfig,
  saveProjectConfig,
  runProjectScaffoldSync,
  RepoNotRegisteredError,
} from "../api";
import type { ScaffoldSyncResponse } from "../api";
import styles from "./ConfigPage.module.css";

export default function ConfigPage() {
  const { project } = useProject();
  const toast = useToast();

  const [savedValue, setSavedValue] = useState("");
  const [draft, setDraft] = useState("");
  const [lastUpdatedAt, setLastUpdatedAt] = useState<Date | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [repoNotRegistered, setRepoNotRegistered] = useState(false);

  const [worktrees, setWorktrees] = useState(false);
  const [trust, setTrust] = useState(false);
  const [syncRunning, setSyncRunning] = useState(false);
  const [syncResult, setSyncResult] = useState<ScaffoldSyncResponse | null>(null);
  const [confirmOpen, setConfirmOpen] = useState(false);

  const loadConfig = useCallback(async () => {
    if (!project) return;
    setLoadError(null);
    setRepoNotRegistered(false);
    try {
      const text = await getProjectConfig(project);
      setSavedValue(text);
      setDraft(text);
      setLastUpdatedAt(new Date());
    } catch (err) {
      if (err instanceof RepoNotRegisteredError) {
        setRepoNotRegistered(true);
      } else {
        setLoadError(err instanceof Error ? err.message : String(err));
      }
    }
  }, [project]);

  useEffect(() => {
    if (!project) return;
    void loadConfig();
  }, [project, loadConfig]);

  const handleSave = async () => {
    if (!project) return;
    setSaving(true);
    setSaveError(null);
    try {
      await saveProjectConfig(project, draft);
      toast.show("config saved - restart daemon and tui to apply");
      // Reload from server so savedValue, draft, and lastUpdatedAt are refreshed
      await loadConfig();
    } catch (err) {
      setSaveError(err instanceof Error ? err.message : String(err));
    } finally {
      setSaving(false);
    }
  };

  const handleRunSync = async () => {
    if (!project) return;
    setSyncRunning(true);
    setSyncResult(null);
    try {
      const result = await runProjectScaffoldSync(project, { worktrees, trust });
      setSyncResult(result);
    } catch (err) {
      setSyncResult({
        ok: false,
        output: "",
        error: err instanceof Error ? err.message : String(err),
      });
    } finally {
      setSyncRunning(false);
    }
  };

  const handleSyncClick = () => {
    if (trust) {
      setConfirmOpen(true);
    } else {
      void handleRunSync();
    }
  };

  // While project is still resolving, render a loading placeholder and skip
  // all API calls.
  if (!project) {
    return (
      <div className={styles.page}>
        <div className={styles.loading}>loading project...</div>
      </div>
    );
  }

  // When the repo is running in bare-db mode, show an empty-state card and
  // suppress the editor and sync controls.
  if (repoNotRegistered) {
    return (
      <div className={styles.page}>
        <div className={styles.emptyCard}>
          config editing requires kas serve --repo &lt;path&gt;. this deployment
          is running in bare-db mode (--db).
        </div>
      </div>
    );
  }

  return (
    <div className={styles.page}>
      {/* Header */}
      <header className={styles.header}>
        <h1 className={styles.title}>config</h1>
        <LastUpdated timestamp={lastUpdatedAt} />
      </header>

      {/* Editor card */}
      <div className={styles.card}>
        {(saveError ?? loadError) && (
          <div className={styles.errorBanner} role="alert">
            {saveError ?? loadError}
          </div>
        )}
        <textarea
          className={styles.editor}
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          spellCheck={false}
          aria-label="config editor"
        />
        <p className={styles.notice}>
          restart daemon and tui to apply saved changes
        </p>
        <div className={styles.buttonRow}>
          <button
            className={styles.btn}
            type="button"
            onClick={() => void loadConfig()}
          >
            reload
          </button>
          <button
            className={`${styles.btn} ${styles.btnPrimary}`}
            type="button"
            disabled={draft === savedValue || saving}
            onClick={() => void handleSave()}
          >
            save
          </button>
        </div>
      </div>

      {/* Scaffold sync card */}
      <div className={styles.card}>
        <h2 className={styles.cardTitle}>scaffold sync</h2>
        <div className={styles.checkboxRow}>
          <label>
            <input
              type="checkbox"
              checked={worktrees}
              onChange={(e) => setWorktrees(e.target.checked)}
            />
            {" "}include worktrees
          </label>
          <label>
            <input
              type="checkbox"
              checked={trust}
              onChange={(e) => setTrust(e.target.checked)}
            />
            {" "}trust project for codex
          </label>
        </div>
        <div>
          <button
            className={`${styles.btn} ${styles.btnPrimary}`}
            type="button"
            disabled={syncRunning}
            onClick={handleSyncClick}
          >
            run sync
          </button>
        </div>
        {syncResult !== null && (
          <div className={styles.syncOutput}>
            {syncResult.error && (
              <div className={styles.syncError}>{syncResult.error}</div>
            )}
            <pre className={syncResult.ok ? styles.syncPre : styles.syncPreError}>
              {syncResult.output}
            </pre>
          </div>
        )}
      </div>

      {/* Trust confirm dialog */}
      <ConfirmDialog
        open={confirmOpen}
        title="trust project for codex"
        message="running scaffold sync with trust=true will mark this project as trusted for codex. continue?"
        confirmLabel="run sync"
        cancelLabel="cancel"
        onConfirm={() => {
          setConfirmOpen(false);
          void handleRunSync();
        }}
        onCancel={() => setConfirmOpen(false)}
      />
    </div>
  );
}
