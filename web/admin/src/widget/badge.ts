import type { MonitorBadge, MonitorScope } from "./host";
import type { MonitorSnapshot } from "./types";

export function deriveBadge(snapshot: MonitorSnapshot | undefined, scope: MonitorScope): MonitorBadge {
  const running = snapshot?.active_agents.filter((agent) => agent.active && !agent.paused).length ?? 0;
  return {
    level: !snapshot || !snapshot.daemon_running ? "offline" : snapshot.attention.length ? "attention" : running ? "running" : "idle",
    running_agents: running,
    blocked: snapshot?.attention.length ?? 0,
    implementing: snapshot?.lifecycle.implementing ?? 0,
    reviewing: snapshot?.lifecycle.reviewing ?? 0,
    project: scope.project,
    task: scope.task,
  };
}
