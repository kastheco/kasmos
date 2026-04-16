"use client";

import { useEffect, useRef, useState } from "react";
import mermaid from "mermaid";

mermaid.initialize({
  startOnLoad: false,
  theme: "dark",
  themeVariables: {
    darkMode: true,
    background: "#1a1a2e",
    primaryColor: "#6366f1",
    primaryTextColor: "#e2e8f0",
    primaryBorderColor: "#4f46e5",
    lineColor: "#64748b",
    secondaryColor: "#1e293b",
    tertiaryColor: "#0f172a",
    noteTextColor: "#e2e8f0",
    noteBkgColor: "#1e293b",
    noteBorderColor: "#334155",
    actorTextColor: "#e2e8f0",
    actorBkg: "#1e293b",
    actorBorder: "#4f46e5",
    signalColor: "#e2e8f0",
    labelBoxBkgColor: "#1e293b",
    labelBoxBorderColor: "#334155",
    labelTextColor: "#e2e8f0",
    sectionBkgColor: "#1e293b",
    altSectionBkgColor: "#0f172a",
    taskBkgColor: "#6366f1",
    taskTextColor: "#e2e8f0",
    activeTaskBkgColor: "#4f46e5",
    activeTaskBorderColor: "#818cf8",
  },
});

let renderCounter = 0;

export function Mermaid({ chart }: { chart: string }) {
  const containerRef = useRef<HTMLDivElement>(null);
  const [svg, setSvg] = useState<string>("");
  const [error, setError] = useState<string>("");

  useEffect(() => {
    const id = `mermaid-${++renderCounter}`;
    mermaid
      .render(id, chart)
      .then(({ svg }) => setSvg(svg))
      .catch((err) => setError(String(err)));
  }, [chart]);

  if (error) {
    return (
      <pre style={{ color: "#f87171", whiteSpace: "pre-wrap" }}>
        {chart}
      </pre>
    );
  }

  return (
    <div
      ref={containerRef}
      className="mermaid-diagram"
      style={{ overflow: "auto", margin: "1.5rem 0" }}
      dangerouslySetInnerHTML={{ __html: svg }}
    />
  );
}
