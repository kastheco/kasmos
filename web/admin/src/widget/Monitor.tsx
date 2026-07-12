import { useEffect, useMemo, useRef, useState } from "react";
import Skeleton from "../components/Skeleton";
import StatusBadge from "../components/StatusBadge";
import AgentList from "./AgentList";
import AttentionList from "./AttentionList";
import EventFeed from "./EventFeed";
import LifecycleRail from "./LifecycleRail";
import { deriveBadge } from "./badge";
import { useMonitorHost } from "./host";
import { useMonitorSnapshot } from "./openai";
import WaveBoard from "./WaveBoard";
import styles from "./widget.module.css";

export default function Monitor() {
  const host = useMonitorHost();
  const mode = host.displayMode;
  const initialProject = host.state?.project ?? host.input?.project ?? host.snapshot?.project ?? host.snapshot?.projects?.[0];
  const initialTask = host.state?.task ?? host.input?.task ?? host.snapshot?.focus?.filename ?? host.snapshot?.tasks?.[0]?.filename;
  const [project, setProject] = useState(initialProject);
  const [task, setTask] = useState(initialTask);
  const { snapshot, stale, phase, refresh } = useMonitorSnapshot(host, project, task);

  useEffect(() => { if (!project && snapshot) setProject(snapshot.project || snapshot.projects?.[0]); }, [project, snapshot]);
  useEffect(() => { if (!task && snapshot && snapshot.project === project) setTask(snapshot.focus?.filename ?? snapshot.tasks?.[0]?.filename); }, [project, task, snapshot]);
  useEffect(() => { void host.saveState?.({ project, task }); }, [host.saveState, project, task]);
  const lastBadge = useRef<string | undefined>(undefined);
  useEffect(() => { if (!snapshot && phase === "loading") return; const badge = deriveBadge(snapshot, { project: project ?? snapshot?.project, task: task ?? snapshot?.focus?.filename }); const serialized = JSON.stringify(badge); if (serialized !== lastBadge.current) { lastBadge.current = serialized; host.setBadge?.(badge); } }, [host.setBadge, phase, project, snapshot, task]);
  const action = host.sendPrompt ? (prompt: string) => { void host.sendPrompt?.(prompt); } : undefined;
  const blockerCount = snapshot?.attention.length ?? 0;
  const runningAgents = useMemo(() => snapshot?.active_agents.filter((agent) => agent.active && !agent.paused).length ?? 0, [snapshot]);

  if (!snapshot && phase === "loading") return <main className={styles.root}><Skeleton variant="text" lines={4} /></main>;
  if (!snapshot && phase === "incompatible") return <main className={styles.root}>monitor bundle / host version mismatch</main>;
  if (!snapshot) return <main className={styles.root}><p>monitor offline</p><button onClick={() => void refresh()}>retry</button></main>;
  return <main className={`${styles.root} ${host.theme === "light" ? styles.light : styles.dark} ${mode === "pip" ? styles.pip : mode === "fullscreen" ? styles.fullscreen : mode === "sidebar" ? styles.sidebar : styles.inlineMode}`} style={mode === "inline" && host.maxHeight ? { maxHeight: host.maxHeight } : undefined}>
    <header className={styles.header}><div><strong>kasmos monitor</strong><span className={styles.connection}>{snapshot.daemon_running ? "● live" : "○ daemon offline"}</span></div><div className={styles.controls}>
      {host.requestDisplayMode && mode !== "sidebar" && mode !== "pip" && <button aria-label="pin as picture in picture" onClick={() => void host.requestDisplayMode?.("pip")}>pin</button>}
      {host.requestDisplayMode && mode !== "sidebar" && mode !== "fullscreen" && <button aria-label="expand monitor" onClick={() => void host.requestDisplayMode?.("fullscreen")}>expand</button>}
    </div></header>
    <div className={styles.stale} aria-live="polite">{stale ? "stale · retrying with last known state" : ""}</div>
    {mode === "pip" ? <div className={styles.pipRail}><LifecycleRail lifecycle={snapshot.lifecycle} /><span>{runningAgents} running</span><StatusBadge status={blockerCount ? `${blockerCount} blocked` : "ready"} /></div> : <>
      <LifecycleRail lifecycle={snapshot.lifecycle} />
      <div className={styles.selectors}>
        {(snapshot.projects?.length ?? 0) > 1 && <label>project<select value={project} onChange={(event) => { setProject(event.target.value); setTask(undefined); }}>{snapshot.projects?.map((name) => <option key={name}>{name}</option>)}</select></label>}
        {(snapshot.tasks?.length ?? 0) > 0 && <label>task<select value={task} onChange={(event) => setTask(event.target.value)}>{snapshot.tasks?.map((item) => <option value={item.filename} key={item.filename}>{item.filename}</option>)}</select></label>}
      </div>
      <section><h2>tasks</h2><ul className={styles.list} aria-label="tasks">{snapshot.tasks?.map((item) => { const percent = item.subtasks_total ? Math.round(item.subtasks_done / item.subtasks_total * 100) : 0; return <li className={styles.task} key={item.filename}><div><button className={styles.linkButton} onClick={() => setTask(item.filename)}>{item.filename}</button><StatusBadge status={item.blocked ? "blocked" : item.status} /></div><progress value={item.subtasks_done} max={item.subtasks_total || 1} aria-label={`${item.filename} progress`} /><small>{percent}% · wave {item.active_wave || 0}/{item.total_waves || 0}</small></li>; })}</ul></section>
      <AttentionList items={snapshot.attention} action={action} />
      {mode === "fullscreen" && <>
        {snapshot.focus && <WaveBoard focus={snapshot.focus} action={action} />}
        <AgentList agents={snapshot.active_agents} />
        {snapshot.focus && <section className={styles.readiness}><h2>readiness</h2><StatusBadge status={snapshot.focus.readiness.status} /><p>review cycle {snapshot.focus.readiness.review_cycle ?? 0}</p><p>checks: {snapshot.focus.readiness.pr_check_status || "not reported"}</p><p>review: {snapshot.focus.readiness.pr_review_decision || "not reported"}</p><p>verification: {snapshot.focus.readiness.last_verify_outcome || "not reported"}</p>{snapshot.focus.readiness.has_review_feedback && action && <button onClick={() => action(`approve review for ${snapshot.focus?.filename}`)}>approve review</button>}</section>}
        <EventFeed events={snapshot.events ?? []} />
      </>}
    </>}
  </main>;
}
