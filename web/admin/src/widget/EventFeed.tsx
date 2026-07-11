import type { EventItem } from "./types";
import styles from "./widget.module.css";
export default function EventFeed({ events }: { events: EventItem[] }) { return <section><h2>events</h2><ol className={styles.list} aria-label="event feed">{events.map((event, index) => <li className={styles.event} key={`${event.at}-${index}`}><time dateTime={event.at}>{new Date(event.at).toLocaleTimeString()}</time><span>{event.message}</span></li>)}</ol></section>; }
