import { type ComponentProps } from "react";
import { useParams } from "react-router";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import type { TaskEntry, SubtaskEntry } from "../types";
import { getTask, getTaskContent, getSubtasks } from "../api";
import { useAutoRefresh } from "../hooks/useAutoRefresh";
import { useProject } from "../hooks/useProject";
import StatusBadge from "../components/StatusBadge";
import MetadataPanel from "../components/MetadataPanel";
import SubtaskProgress from "../components/SubtaskProgress";
import LastUpdated from "../components/LastUpdated";
import Skeleton from "../components/Skeleton";
import styles from "./TaskDetailPage.module.css";

type TaskDetailData = {
  task: TaskEntry;
  content: string;
  subtasks: SubtaskEntry[];
};

const mdComponents: ComponentProps<typeof ReactMarkdown>["components"] = {
  input({ node: _node, ...props }) {
    if (props.type === "checkbox") {
      return <input {...props} disabled />;
    }
    return <input {...props} />;
  },
  pre({ node: _node, children, ...rest }) {
    const child = Array.isArray(children) ? children[0] : children;
    const className =
      child && typeof child === "object" && "props" in child
        ? (child.props as { className?: string }).className ?? ""
        : "";
    const match = /language-(\w+)/.exec(className);
    if (match) {
      return (
        <div className={styles.codeBlock}>
          <span className={styles.codeLangLabel}>{match[1]}</span>
          <pre {...rest}>{children}</pre>
        </div>
      );
    }
    return <pre {...rest}>{children}</pre>;
  },
  code({ node: _node, className, children, ...rest }) {
    return (
      <code className={className} {...rest}>
        {children}
      </code>
    );
  },
};

export default function TaskDetailPage() {
  const { filename: rawFilename } = useParams<{ filename: string }>();
  const filename = rawFilename ? decodeURIComponent(rawFilename) : undefined;

  const { project } = useProject();

  const { data, loading, error, lastUpdatedAt, isRefreshing } =
    useAutoRefresh<TaskDetailData | null>(
      async () => {
        if (!filename) throw new Error("no task filename provided");
        if (!project) return null;
        const [task, content, subtasks] = await Promise.all([
          getTask(project, filename),
          getTaskContent(project, filename),
          getSubtasks(project, filename),
        ]);
        return { task, content, subtasks };
      },
      [project, filename],
    );

  if (!filename) {
    return (
      <div className={styles.errorPanel}>
        <p className={styles.errorText}>no task filename provided</p>
      </div>
    );
  }

  if (loading) {
    return (
      <div className={styles.page}>
        <header className={styles.header}>
          <Skeleton variant="text" lines={1} />
        </header>
        <div className={styles.layout}>
          <section className={styles.main}>
            <Skeleton variant="block" />
          </section>
          <aside className={styles.sidebar}>
            <Skeleton variant="card" />
            <Skeleton variant="card" />
          </aside>
        </div>
      </div>
    );
  }

  if (error || !data) {
    return (
      <div className={styles.errorPanel}>
        <p className={styles.errorText}>{error ?? "task not found"}</p>
      </div>
    );
  }

  const { task, content, subtasks } = data;

  return (
    <div className={styles.page}>
      <header className={styles.header}>
        <h1 className={styles.title}>{filename}</h1>
        <div className={styles.badges}>
          <StatusBadge status={task.status} />
          {task.topic && task.topic.trim() !== "" && (
            <span className={styles.topicPill}>{task.topic}</span>
          )}
          <LastUpdated timestamp={lastUpdatedAt} isRefreshing={isRefreshing} />
        </div>
      </header>

      <div className={styles.layout}>
        <section className={styles.main}>
          <div className={styles.markdown}>
            <ReactMarkdown
              remarkPlugins={[remarkGfm]}
              components={mdComponents}
            >
              {content}
            </ReactMarkdown>
          </div>
        </section>

        <aside className={styles.sidebar}>
          <MetadataPanel task={task} />
          <SubtaskProgress subtasks={subtasks} />
        </aside>
      </div>
    </div>
  );
}
