"use client";

import styles from "../page.module.css";
import StarField from "./StarField";
import TypewriterText from "./TypewriterText";
import ScrollReveal from "./ScrollReveal";
import FeatureCard from "./FeatureCard";
import InstallTabs from "./InstallTabs";
import Header from "./Header";
import OrchestrationVisual from "./OrchestrationVisual";

const features = [
  {
    icon: "\u{1F916}",
    title: "multi-agent orchestration",
    description:
      "run claude, codex, gemini, amp, and other AI agents concurrently. each gets an isolated git worktree and tmux session.",
  },
  {
    icon: "\u{1F30A}",
    title: "wave-based lifecycle",
    description:
      "plans decompose into waves of parallel tasks. an architect pass structures each wave before coders implement. reviewers verify the work, and fixers apply requested changes — looping back to review until the task is clean.",
  },
  {
    icon: "\u{1F527}",
    title: "MCP server architecture",
    description:
      "kasmos exposes an MCP server for task CRUD, signals, instance management, and codebase tools. agents interact through MCP, not filesystem hacks.",
  },
  {
    icon: "\u{1F5C2}",
    title: "multi-repo support",
    description:
      "manage tasks across multiple repositories from a single daemon. each repo gets its own config and task store.",
  },
  {
    icon: "\u{1F4BE}",
    title: "session persistence",
    description:
      "sessions survive restarts. pick up where you left off, even after rebooting your machine.",
  },
  {
    icon: "\u{1F680}",
    title: "auto-commit & PR",
    description:
      "automatically commit agent work and create pull requests. ship faster with less manual overhead.",
  },
];

const typewriterTexts = [
  "run agents in parallel across isolated worktrees",
  "wave-based execution with lifecycle signals",
  "MCP-powered orchestration built-in",
  "concurrent agents, zero conflicts",
];

export default function PageContent() {
  return (
    <div className={styles.page}>
      <StarField />

      <div className={`${styles.glowOrb} ${styles.glowAmber}`} />
      <div className={`${styles.glowOrb} ${styles.glowTeal}`} />

      <div className={styles.content}>
        <Header />

        {/* Hero */}
        <section className={styles.hero}>
          <OrchestrationVisual className={styles.heroBee} />
          <h1 className={styles.heroTitle}>
            <img
              src="/logo-full.png"
              alt="kasmos"
              className={styles.heroWordmark}
              width={600}
              height={338}
            />
          </h1>
          <p className={styles.heroSubtitle}>
            multi-agent orchestration for your terminal
          </p>
          <div className={styles.heroTypewriter}>
            <TypewriterText texts={typewriterTexts} />
          </div>
          <div className={styles.heroCtas}>
            <a href="#install" className={styles.ctaPrimary}>
              install now
            </a>
            <a
              href="https://github.com/kastheco/kasmos"
              target="_blank"
              rel="noopener noreferrer"
              className={styles.ctaSecondary}
            >
              view on github
            </a>
            <a
              href="/docs"
              className={styles.ctaSecondary}
            >
              read the docs
            </a>
          </div>
        </section>

        {/* Features */}
        <section className={styles.section}>
          <ScrollReveal>
            <h2 className={styles.sectionTitle}>why <span className={styles.gradientInline}>kasmos</span>?</h2>
            <p className={styles.sectionSubtitle}>
              wave-based execution, isolated worktrees, and MCP-native tooling — everything you need to run concurrent AI agents at scale.
            </p>
          </ScrollReveal>
          <div className={styles.featuresGrid}>
            {features.map((feature, i) => (
              <ScrollReveal key={feature.title} delay={i * 0.1}>
                <FeatureCard {...feature} />
              </ScrollReveal>
            ))}
          </div>
        </section>

        {/* How it Works */}
        <section className={styles.section}>
          <ScrollReveal>
            <h2 className={styles.sectionTitle}>how it works</h2>
            <p className={styles.sectionSubtitle}>
              kasmos decomposes your feature spec into waves of parallel tasks,
              each executed by a specialized agent in its own isolated worktree.
            </p>
          </ScrollReveal>

          <ScrollReveal>
            <div className={styles.lifecyclePipeline}>
              {(
                [
                  { label: "planning", color: "amber", arrow: "→" },
                  { label: "architect", color: "teal", arrow: "→" },
                  { label: "implementing", color: "teal", arrow: "→" },
                  { label: "reviewing", color: "amber", arrow: "⇄", note: "review/fix loop" },
                  { label: "fixer", color: "amber", arrow: "→" },
                  { label: "done", color: "teal" },
                ] as const
              ).map((step, i, arr) => (
                <div key={step.label} className={styles.lifecycleStepGroup}>
                  <div
                    className={`${styles.lifecycleStep} ${step.color === "amber" ? styles.lifecycleAmber : styles.lifecycleTeal}`}
                  >
                    <span className={styles.lifecycleStepNum}>{i + 1}</span>
                    <span className={styles.lifecycleStepLabel}>{step.label}</span>
                  </div>
                  {i < arr.length - 1 && (
                    <div className={`${styles.lifecycleArrow}${"arrow" in step && step.arrow === "⇄" ? ` ${styles.lifecycleArrowLoop}` : ""}`}>
                      {"arrow" in step ? step.arrow : "→"}
                      {"note" in step && step.note ? (
                        <span className={styles.lifecycleLoopNote}>{step.note}</span>
                      ) : null}
                    </div>
                  )}
                </div>
              ))}
            </div>
          </ScrollReveal>

          <div className={styles.agentsGrid}>
            {(
              [
                {
                  role: "planner",
                  icon: "📋",
                  desc: "writes the feature spec and breaks it into high-level tasks.",
                  color: "amber",
                },
                {
                  role: "architect",
                  icon: "🏗️",
                  desc: "decomposes the plan into implementation waves with inter-task dependencies.",
                  color: "teal",
                },
                {
                  role: "coder",
                  icon: "⚡",
                  desc: "implements the task in an isolated git worktree, following TDD.",
                  color: "teal",
                },
                {
                  role: "reviewer",
                  icon: "🔍",
                  desc: "checks correctness, spec compliance, and code quality per task.",
                  color: "amber",
                },
                {
                  role: "fixer",
                  icon: "🔧",
                  desc: "applies reviewer feedback, debugs issues, and prepares the work for re-review.",
                  color: "amber",
                },
                /* temporarily hidden — master role not currently active in the default pipeline */
                // {
                //   role: "master",
                //   icon: "🎯",
                //   desc: "Performs final holistic review across all merged changes before shipping.",
                //   color: "teal",
                // },
              ] as const
            ).map((agent, i) => (
              <ScrollReveal key={agent.role} delay={i * 0.08}>
                <div
                  className={`${styles.agentCard} ${agent.color === "amber" ? styles.agentAmber : styles.agentTeal}`}
                >
                  <span className={styles.agentIcon}>{agent.icon}</span>
                  <span className={styles.agentRole}>{agent.role}</span>
                  <p className={styles.agentDesc}>{agent.desc}</p>
                </div>
              </ScrollReveal>
            ))}
          </div>
        </section>

        {/* Installation */}
        <section id="install" className={styles.section}>
          <ScrollReveal className={styles.installSection}>
            <h2 className={styles.sectionTitle}>get started</h2>
            <p className={styles.sectionSubtitle}>
              install kasmos in seconds. works on macOS, Linux, and Windows.
            </p>
            <InstallTabs />
            <p className={styles.installPrereqs}>
              prerequisites: tmux, gh, and at least one supported AI CLI —{" "}
              <a href="/docs/getting-started/prerequisites/">see full list</a>
            </p>
          </ScrollReveal>
        </section>

        {/* Footer */}
        <footer className={styles.footer}>
          <div className={styles.footerGradientLine} />
          <p className={styles.footerText}>
            &copy; {new Date().getFullYear()} kasmos by{" "}
            <a
              href="https://github.com/kastheco"
              target="_blank"
              rel="noopener noreferrer"
            >
              kastheco
            </a>
            . licensed under{" "}
            <a
              href="https://github.com/kastheco/kasmos/blob/main/LICENSE.md"
              target="_blank"
              rel="noopener noreferrer"
            >
              BSL 1.1
            </a>
          </p>
        </footer>
      </div>
    </div>
  );
}
