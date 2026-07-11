import type { AttentionItem } from "./types";
import styles from "./widget.module.css";

export default function AttentionList({ items, action }: { items: AttentionItem[]; action?: (prompt: string) => void }) {
  return <section><h2>attention</h2>{items.length === 0 ? <p className={styles.muted}>nothing needs attention</p> : <ul className={styles.list} aria-label="attention items">{items.map((item, index) => <li className={styles.attention} key={`${item.task}-${item.kind}-${index}`}><div><strong>{item.kind.replace(/_/g, " ")}</strong> · {item.task}</div>{item.detail && <p>{item.detail}</p>}{action && <button onClick={() => action(`look at the blocker on ${item.task}`)}>look at blocker</button>}</li>)}</ul>}</section>;
}
