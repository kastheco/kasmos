import styles from "./LifecycleDiagram.module.css";

interface LifecycleDiagramProps {
  className?: string;
}

export default function LifecycleDiagram({ className }: LifecycleDiagramProps) {
  return (
    <figure className={`${styles.diagram} ${className ?? ""}`}>
      <img
        className={styles.image}
        src="/kasmos-lifecycle-flow.svg"
        alt="kasmos lifecycle: optional planner drafts feed the architect, then coder waves, review and fix loop, readiness, and done."
      />
    </figure>
  );
}
