import { useState } from "react";
import MDEditor from "@uiw/react-md-editor";
import "@uiw/react-md-editor/markdown-editor.css";
import styles from "./PlanEditor.module.css";

interface PlanEditorProps {
  initialValue: string;
  onSave: (value: string) => Promise<void>;
  onCancel: () => void;
}

export default function PlanEditor({
  initialValue,
  onSave,
  onCancel,
}: PlanEditorProps) {
  const [draft, setDraft] = useState(initialValue);
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);

  async function handleSave() {
    setSaving(true);
    setSaveError(null);
    try {
      await onSave(draft);
    } catch (err) {
      setSaveError(err instanceof Error ? err.message : "save failed");
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className={styles.container} data-color-mode="dark">
      <div className={styles.editorWrap}>
        <MDEditor
          value={draft}
          onChange={(val) => setDraft(val ?? "")}
          height="100%"
          preview="edit"
          visibleDragbar={false}
        />
      </div>

      {saveError && (
        <p className={styles.error} role="alert">
          {saveError}
        </p>
      )}

      <div className={styles.actions}>
        <button
          className={styles.cancelBtn}
          onClick={onCancel}
          disabled={saving}
          type="button"
        >
          cancel
        </button>
        <button
          className={styles.saveBtn}
          onClick={handleSave}
          disabled={saving}
          type="button"
        >
          {saving ? "saving…" : "save"}
        </button>
      </div>
    </div>
  );
}
