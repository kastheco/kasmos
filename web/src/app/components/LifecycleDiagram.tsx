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
        alt="kasmos lifecycle: planner and architect baseline run in parallel, then architect merge, coder waves, review and fix loop, readiness, and done."
      />
    </figure>
  );
}
