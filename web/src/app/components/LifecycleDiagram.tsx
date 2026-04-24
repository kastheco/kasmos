import styles from "./LifecycleDiagram.module.css";

interface LifecycleDiagramProps {
  className?: string;
}

export default function LifecycleDiagram({ className }: LifecycleDiagramProps) {
  return (
    <figure
      className={`${styles.diagram} ${className ?? ""}`}
      tabIndex={0}
      aria-label="kasmos lifecycle: planner and architect baseline run in parallel, then architect merge, coder waves, review and fix loop, readiness, and done."
    >
      <img
        className={styles.image}
        src="/kasmos-lifecycle-flow.svg"
        alt=""
        aria-hidden="true"
      />
    </figure>
  );
}
