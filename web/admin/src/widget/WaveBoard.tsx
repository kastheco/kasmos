import StatusBadge from "../components/StatusBadge";
import type { TaskFocus } from "./types";
import styles from "./widget.module.css";

export default function WaveBoard({ focus, action }: { focus: TaskFocus; action?: (prompt: string) => void }) {
  return <section><h2>waves</h2><div className={styles.waveBoard} role="list">
    {focus.waves.map((wave) => <article role="listitem" className={styles.card} key={wave.wave}><header><strong>wave {wave.wave}</strong>{wave.active && <span> active</span>}</header>
      <ul>{wave.tasks.map((task) => <li key={task.number}><span>{task.number}. {task.title}</span><StatusBadge status={task.status} /></li>)}</ul>
      {wave.active && action && <button onClick={() => action(`start wave ${wave.wave} on ${focus.filename}`)}>start wave {wave.wave}</button>}
    </article>)}
  </div></section>;
}
