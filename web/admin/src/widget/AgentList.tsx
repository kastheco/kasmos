import StatusBadge from "../components/StatusBadge";
import type { ActiveAgent } from "./types";
import styles from "./widget.module.css";

export default function AgentList({ agents }: { agents: ActiveAgent[] }) {
  return <section><h2>agents</h2><ul className={styles.list} aria-label="active agents">{agents.map((agent, index) => <li className={styles.card} key={`${agent.task}-${agent.role}-${index}`}>
    <div><strong>{agent.role}</strong> · {agent.task}{agent.wave ? ` · wave ${agent.wave}` : ""}{agent.task_number ? ` task ${agent.task_number}` : ""}</div>
    <div className={styles.meta}>{agent.branch || "no branch"} · {agent.worktree || "no worktree"} · {agent.last_activity ? new Date(agent.last_activity).toLocaleTimeString() : "no activity"}</div>
    <StatusBadge status={agent.paused ? "paused" : agent.stage || (agent.active ? "running" : "ready")} />
  </li>)}</ul></section>;
}
