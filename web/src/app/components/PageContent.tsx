"use client";

import styles from "../page.module.css";
import StarField from "./StarField";
import GradientText from "./GradientText";
import TypewriterText from "./TypewriterText";
import ScrollReveal from "./ScrollReveal";
import FeatureCard from "./FeatureCard";
import InstallTabs from "./InstallTabs";
import Header from "./Header";
import PixelBee from "./PixelBee";

const features = [
  {
    icon: "\u{1F916}",
    title: "Multi-Agent Orchestration",
    description:
      "Run claude, codex, gemini, amp, and other AI agents concurrently. Each gets an isolated git worktree and tmux session.",
  },
  {
    icon: "\u{1F30A}",
    title: "Wave-Based Lifecycle",
    description:
      "Plans decompose into waves of parallel tasks. An architect pass structures each wave before coders implement. Reviewers verify the work, and fixers apply requested changes — looping back to review until the task is clean.",
  },
  {
    icon: "\u{1F527}",
    title: "MCP Server Architecture",
    description:
      "kasmos exposes an MCP server for task CRUD, signals, instance management, and codebase tools. Agents interact through MCP, not filesystem hacks.",
  },
  {
    icon: "\u{1F5C2}",
    title: "Multi-Repo Support",
    description:
      "Manage tasks across multiple repositories from a single daemon. Each repo gets its own config and task store.",
  },
  {
    icon: "\u{1F4BE}",
    title: "Session Persistence",
    description:
      "Sessions survive restarts. Pick up where you left off, even after rebooting your machine.",
  },
  {
    icon: "\u{1F680}",
    title: "Auto-commit & PR",
    description:
      "Automatically commit agent work and create pull requests. Ship faster with less manual overhead.",
  },
];

const typewriterTexts = [
  "Run agents in parallel across isolated worktrees",
  "Wave-based execution with lifecycle signals",
  "MCP-powered orchestration built-in",
  "Concurrent agents, zero conflicts",
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
          <PixelBee scale={3} bob className={styles.heroBee} />
          <GradientText as="h1" className={styles.heroTitle}>
            kasmos
          </GradientText>
          <p className={styles.heroSubtitle}>
            multi-agent orchestration for your terminal
          </p>
          <div className={styles.heroTypewriter}>
            <TypewriterText texts={typewriterTexts} />
          </div>
          <div className={styles.heroCtas}>
            <a href="#install" className={styles.ctaPrimary}>
              Install Now
            </a>
            <a
              href="https://github.com/kastheco/kasmos"
              target="_blank"
              rel="noopener noreferrer"
              className={styles.ctaSecondary}
            >
              View on GitHub
            </a>
            <a
              href="/docs"
              className={styles.ctaSecondary}
            >
              Read the Docs
            </a>
          </div>
        </section>

        {/* Features */}
        <section className={styles.section}>
          <ScrollReveal>
            <h2 className={styles.sectionTitle}>Why kasmos?</h2>
            <p className={styles.sectionSubtitle}>
              Wave-based execution, isolated worktrees, and MCP-native tooling — everything you need to run concurrent AI agents at scale.
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
            <h2 className={styles.sectionTitle}>How it Works</h2>
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
                  desc: "Writes the feature spec and breaks it into high-level tasks.",
                  color: "amber",
                },
                {
                  role: "architect",
                  icon: "🏗️",
                  desc: "Decomposes the plan into implementation waves with inter-task dependencies.",
                  color: "teal",
                },
                {
                  role: "coder",
                  icon: "⚡",
                  desc: "Implements the task in an isolated git worktree, following TDD.",
                  color: "teal",
                },
                {
                  role: "reviewer",
                  icon: "🔍",
                  desc: "Checks correctness, spec compliance, and code quality per task.",
                  color: "amber",
                },
                {
                  role: "fixer",
                  icon: "🔧",
                  desc: "Applies reviewer feedback, debugs issues, and prepares the work for re-review.",
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
            <h2 className={styles.sectionTitle}>Get Started</h2>
            <p className={styles.sectionSubtitle}>
              Install kasmos in seconds. Works on macOS, Linux, and Windows.
            </p>
            <InstallTabs />
            <p className={styles.installPrereqs}>
              Prerequisites: tmux, gh (GitHub CLI)
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
            . Licensed under{" "}
            <a
              href="https://github.com/kastheco/kasmos/blob/main/LICENSE.md"
              target="_blank"
              rel="noopener noreferrer"
            >
              MIT License
            </a>
          </p>
        </footer>
      </div>
    </div>
  );
}
