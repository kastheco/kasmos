import type { ComponentProps } from "react";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import type {
  ArchitectDecisionAuditResponse,
  ArchitectDecisionDifference,
  ArchitectPlannerDraftDecision,
} from "../types";
import styles from "./ArchitectDecisionPanel.module.css";

export interface ArchitectDecisionPanelProps {
  response: ArchitectDecisionAuditResponse | null;
  loading?: boolean;
  error?: Error | null;
}

const mdComponents: ComponentProps<typeof ReactMarkdown>["components"] = {
  a({ node: _node, href, children, ...rest }) {
    const external =
      href?.startsWith("http://") || href?.startsWith("https://");
    if (external) {
      return (
        <a href={href} target="_blank" rel="noopener noreferrer" {...rest}>
          {children}
        </a>
      );
    }
    return (
      <a href={href} {...rest}>
        {children}
      </a>
    );
  },
};

function MarkdownBlock({ value }: { value: string }) {
  return (
    <div className={styles.markdown}>
      <ReactMarkdown remarkPlugins={[remarkGfm]} components={mdComponents}>
        {value}
      </ReactMarkdown>
    </div>
  );
}

function EmptyState({ reason }: { reason?: string }) {
  const copy =
    reason === "architect_not_run"
      ? "architect has not run for this task"
      : reason === "decision_audit_missing"
        ? "architect decision audit is not available"
        : reason === "repo_not_registered"
          ? "project repository is not registered"
          : "architect decision audit is not available";

  return (
    <section className={styles.panel} aria-label="architect decisions">
      <p className={styles.empty}>{copy}</p>
    </section>
  );
}

function TextSection({
  title,
  children,
}: {
  title: string;
  children?: string;
}) {
  if (!children || children.trim() === "") return null;
  return (
    <section className={styles.section}>
      <h3 className={styles.sectionTitle}>{title}</h3>
      <p className={styles.bodyText}>{children}</p>
    </section>
  );
}

function formatList(values?: string[] | number[]): string {
  if (!values || values.length === 0) return "-";
  return values.join(", ");
}

function PlannerDraftsTable({
  drafts,
}: {
  drafts?: ArchitectPlannerDraftDecision[];
}) {
  if (!drafts || drafts.length === 0) return null;
  return (
    <section className={styles.section}>
      <h3 className={styles.sectionTitle}>planner drafts</h3>
      <div className={styles.tableScroller}>
        <table className={styles.table}>
          <thead>
            <tr>
              <th>profile</th>
              <th>decision</th>
              <th>summary</th>
              <th>rationale</th>
            </tr>
          </thead>
          <tbody>
            {drafts.map((draft, index) => (
              <tr key={`${draft.profile}-${index}`}>
                <td>
                  <strong>{draft.profile}</strong>
                </td>
                <td>{draft.decision}</td>
                <td>{draft.summary || "-"}</td>
                <td>{draft.rationale || "-"}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>
  );
}

function DifferencesTable({
  differences,
}: {
  differences?: ArchitectDecisionDifference[];
}) {
  if (!differences || differences.length === 0) return null;
  return (
    <section className={styles.section}>
      <h3 className={styles.sectionTitle}>differences</h3>
      <div className={styles.tableScroller}>
        <table className={styles.table}>
          <thead>
            <tr>
              <th>area</th>
              <th>planner proposal</th>
              <th>baseline proposal</th>
              <th>final decision</th>
              <th>rationale</th>
              <th>scope</th>
            </tr>
          </thead>
          <tbody>
            {differences.map((diff, index) => (
              <tr key={`${diff.area}-${index}`}>
                <td>
                  <strong>{diff.area}</strong>
                  {(diff.related_files?.length ||
                    diff.task_numbers?.length) && (
                    <span className={styles.metaLine}>
                      {formatList(diff.related_files)}
                      {diff.task_numbers?.length
                        ? ` - tasks ${formatList(diff.task_numbers)}`
                        : ""}
                    </span>
                  )}
                </td>
                <td>{diff.planner_proposal || "-"}</td>
                <td>{diff.baseline_proposal || "-"}</td>
                <td>{diff.final_decision}</td>
                <td>{diff.rationale || "-"}</td>
                <td>{diff.scope || "-"}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>
  );
}

function Timestamps({
  response,
}: {
  response: ArchitectDecisionAuditResponse;
}) {
  const rows = [
    ["architect meta", response.timestamps?.architect_meta_at],
    [
      "decision audit created",
      response.timestamps?.decision_audit_created_at ??
        response.decision_audit?.created_at,
    ],
  ].filter((row): row is [string, string] => Boolean(row[1]));

  if (rows.length === 0) return null;
  return (
    <dl className={styles.timestamps} aria-label="timestamps">
      {rows.map(([label, value]) => (
        <div key={label} className={styles.timestampRow}>
          <dt>{label}</dt>
          <dd>{value}</dd>
        </div>
      ))}
    </dl>
  );
}

export default function ArchitectDecisionPanel({
  response,
  loading = false,
  error = null,
}: ArchitectDecisionPanelProps) {
  if (loading) {
    return (
      <section className={styles.panel} aria-label="architect decisions">
        <p className={styles.empty}>loading architect decisions</p>
      </section>
    );
  }

  if (error && !response) {
    return (
      <section className={styles.panel} aria-label="architect decisions">
        <p className={styles.error}>
          could not load architect decisions: {error.message}
        </p>
      </section>
    );
  }

  if (!response) {
    return <EmptyState />;
  }

  if (!response.available) {
    return <EmptyState reason={response.reason} />;
  }

  const audit = response.decision_audit;
  const finalDecision = audit?.final_decision || response.final_markdown;
  const showFinalMarkdown =
    Boolean(response.final_markdown) &&
    response.final_markdown !== audit?.final_decision;

  return (
    <article className={styles.panel} aria-label="architect decisions">
      <header className={styles.header}>
        <h2 className={styles.title}>architect decisions</h2>
        {audit?.baseline_source && (
          <span className={styles.source}>{audit.baseline_source}</span>
        )}
      </header>
      {error && (
        <p className={styles.error}>
          could not refresh architect decisions: {error.message}
        </p>
      )}

      <TextSection title="summary">{audit?.summary}</TextSection>
      <TextSection title="planner summary">{audit?.planner_summary}</TextSection>
      <TextSection title="architect baseline summary">
        {audit?.baseline_summary}
      </TextSection>

      {finalDecision && (
        <section className={styles.section}>
          <h3 className={styles.sectionTitle}>final decision</h3>
          <MarkdownBlock value={finalDecision} />
        </section>
      )}

      <PlannerDraftsTable drafts={audit?.planner_drafts} />

      <DifferencesTable differences={audit?.differences} />

      {showFinalMarkdown && response.final_markdown && (
        <section className={styles.section}>
          <h3 className={styles.sectionTitle}>final markdown</h3>
          <MarkdownBlock value={response.final_markdown} />
        </section>
      )}

      <Timestamps response={response} />
    </article>
  );
}
