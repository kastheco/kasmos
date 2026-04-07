"use client";

import { useState } from "react";
import styles from "./InstallTabs.module.css";

interface CommandInstallMethod {
  label: string;
  kind: "command";
  command: string;
}

interface LinkInstallMethod {
  label: string;
  kind: "link";
  href: string;
  description: string;
  ctaLabel: string;
}

type InstallMethod = CommandInstallMethod | LinkInstallMethod;

const installMethods: InstallMethod[] = [
  {
    label: "homebrew",
    kind: "command",
    command: "brew install kastheco/tap/kasmos",
  },
  {
    label: "go install",
    kind: "command",
    command: "go install github.com/kastheco/kasmos@latest",
  },
  {
    label: "shell script",
    kind: "command",
    command: "curl -fsSL https://raw.githubusercontent.com/kastheco/kasmos/main/install.sh | bash",
  },
  {
    label: "github releases",
    kind: "link",
    href: "https://github.com/kastheco/kasmos/releases/latest",
    description: "download a pre-built binary for your platform from the github releases page.",
    ctaLabel: "view releases",
  },
];

export default function InstallTabs() {
  const [activeTab, setActiveTab] = useState(0);
  const [copied, setCopied] = useState(false);

  const active = installMethods[activeTab];

  const handleCopy = async () => {
    if (active.kind !== "command") return;
    await navigator.clipboard.writeText(active.command);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div className={styles.terminal}>
      <div className={styles.titleBar}>
        <span className={`${styles.dot} ${styles.dotRed}`} />
        <span className={`${styles.dot} ${styles.dotYellow}`} />
        <span className={`${styles.dot} ${styles.dotGreen}`} />
      </div>
      <div className={styles.tabs}>
        {installMethods.map((method, i) => (
          <button
            key={method.label}
            className={`${styles.tab} ${i === activeTab ? styles.tabActive : ""}`}
            onClick={() => { setActiveTab(i); setCopied(false); }}
          >
            {method.label}
          </button>
        ))}
      </div>
      <div className={styles.content}>
        {active.kind === "command" ? (
          <pre className={styles.command}>
            {active.command.split("\n").map((line, i, arr) => (
              <span key={i}>
                <span className={styles.prompt}>$ </span>{line}
                {i < arr.length - 1 && "\n"}
              </span>
            ))}
          </pre>
        ) : (
          <div className={styles.releasesPanel}>
            <p className={styles.releasesDescription}>{active.description}</p>
            <a
              href={active.href}
              target="_blank"
              rel="noopener noreferrer"
              className={styles.releasesLink}
            >
              {active.ctaLabel} →
            </a>
          </div>
        )}
      </div>
      {active.kind === "command" && (
        <div className={styles.copyRow}>
          <button
            className={`${styles.copyBtn} ${copied ? styles.copied : ""}`}
            onClick={handleCopy}
          >
            {copied ? (
              <>
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><polyline points="20 6 9 17 4 12"/></svg>
                copied
              </>
            ) : (
              <>
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><rect x="9" y="9" width="13" height="13" rx="2" ry="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/></svg>
                copy
              </>
            )}
          </button>
        </div>
      )}
    </div>
  );
}
