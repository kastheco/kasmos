import { useEffect, useState } from "react";
import styles from "./LastUpdated.module.css";

interface LastUpdatedProps {
  timestamp: Date | null;
  isRefreshing?: boolean;
  className?: string;
}

function formatAge(timestamp: Date): string {
  const diffMs = Date.now() - timestamp.getTime();
  const diffSec = Math.round(diffMs / 1000);

  if (diffSec < 5) return "updated just now";
  if (diffSec < 60) return `updated ${diffSec} seconds ago`;

  const diffMin = Math.round(diffSec / 60);
  if (diffMin < 60) return `updated ${diffMin} minute${diffMin === 1 ? "" : "s"} ago`;

  const diffHr = Math.round(diffMin / 60);
  if (diffHr < 24) return `updated ${diffHr} hour${diffHr === 1 ? "" : "s"} ago`;

  const diffDay = Math.round(diffHr / 24);
  return `updated ${diffDay} day${diffDay === 1 ? "" : "s"} ago`;
}

export default function LastUpdated({
  timestamp,
  isRefreshing = false,
  className,
}: LastUpdatedProps) {
  const [, setTick] = useState(0);

  useEffect(() => {
    if (!timestamp) return;
    const id = setInterval(() => setTick((t) => t + 1), 1000);
    return () => clearInterval(id);
  }, [timestamp]);

  const classes = [styles.root, className].filter(Boolean).join(" ");

  if (isRefreshing) {
    return <span className={classes}>updating...</span>;
  }

  if (!timestamp) return null;

  return <span className={classes}>{formatAge(timestamp)}</span>;
}
