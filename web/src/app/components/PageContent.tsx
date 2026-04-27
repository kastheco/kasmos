"use client";

import styles from "../page.module.css";
import StarField from "./StarField";
import TypewriterText from "./TypewriterText";
import ScrollReveal from "./ScrollReveal";
import FeatureCard from "./FeatureCard";
import InstallTabs from "./InstallTabs";
import Header from "./Header";
import OrchestrationVisual from "./OrchestrationVisual";
import LifecycleDiagram from "./LifecycleDiagram";

const features = [
  {
    icon: "\u{1F916}",
    title: "multi-agent orchestration",
    description:
      "run claude, codex, gemini, amp, and other AI agents concurrently. each gets an isolated git worktree and tmux session.",
    tone: "iris",
  },
  {
    icon: "\u{1F30A}",
    title: "wave-based lifecycle",
    description:
      "plans decompose into waves of parallel tasks. an architect pass structures each wave before coders implement. reviewers verify the work, fixers apply requested changes — looping back until the task is clean — and a master agent performs a final readiness check before the task ships.",
    tone: "foam",
  },
  {
    icon: "\u{1F527}",
    title: "mcp server architecture",
    description:
      "kasmos exposes a shared http mcp endpoint for task crud, signals, instance management, and codebase tools. agents use mcp by default, not ad hoc cli wrappers or filesystem hacks.",
    tone: "gold",
  },
  {
    icon: "\u{1F5C2}",
    title: "multi-repo support",
    description:
      "manage tasks across multiple repositories from a single daemon while sharing one global task store. repo config stays local; task state stays centralized.",
    tone: "rose",
  },
  {
    icon: "\u{1F4BE}",
    title: "session persistence",
    description:
      "sessions survive restarts. pick up where you left off, even after rebooting your machine.",
    tone: "foam",
  },
  {
    icon: "\u{1F680}",
    title: "auto-commit & PR",
    description:
      "automatically commit agent work and create pull requests. ship faster with less manual overhead.",
    tone: "gold",
  },
 ] as const;

const agents = [
  {
    role: "planner",
    icon: "📋",
    desc: "writes the feature spec and breaks it into high-level tasks.",
    color: "gold",
  },
  {
    role: "architect",
    icon: "🏗️",
    desc: "decomposes the plan into implementation waves with inter-task dependencies.",
    color: "iris",
  },
  {
    role: "coder",
    icon: "⚡",
    desc: "implements the task in an isolated git worktree, following TDD.",
    color: "foam",
  },
  {
    role: "reviewer",
    icon: "🔍",
    desc: "checks correctness, spec compliance, and code quality per task.",
    color: "rose",
  },
  {
    role: "fixer",
    icon: "🔧",
    desc: "applies reviewer feedback, debugs issues, and prepares the work for re-review.",
    color: "gold",
  },
  {
    role: "master",
    icon: "🎯",
    desc: "final readiness gate after reviewer approval — performs holistic review across all merged changes before marking the task done.",
    color: "pine",
  },
] as const;

const typewriterTexts = [
  "parallel agents, isolated worktrees",
  "wave-based execution with signals",
  "mcp-powered orchestration",
  "concurrent agents, zero conflicts",
];

export default function PageContent() {
  const agentToneClasses = {
    gold: styles.agentGold,
    iris: styles.agentIris,
    foam: styles.agentFoam,
    rose: styles.agentRose,
    pine: styles.agentPine,
  } as const;

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
          <p className={styles.heroKicker}>mcp-first orchestration system</p>
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
          <div className={styles.heroSignals}>
            <span className={`${styles.heroSignal} ${styles.signalGold}`}>
              planner → architect
            </span>
            <span className={`${styles.heroSignal} ${styles.signalIris}`}>
              parallel worktrees
            </span>
            <span className={`${styles.heroSignal} ${styles.signalFoam}`}>
              review / fix loop
            </span>
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
          <div className={`${styles.sectionFrame} ${styles.sectionFrameIris}`}>
            <ScrollReveal>
              <p className={styles.sectionEyebrow}>capability map</p>
              <h2 className={styles.sectionTitle}>why <span className={styles.gradientInline}>kasmos</span>?</h2>
              <p className={styles.sectionSubtitle}>
                wave-based execution, isolated worktrees, and mcp-native tooling — everything you need to run concurrent ai agents at scale.
              </p>
            </ScrollReveal>
            <div className={styles.featuresGrid}>
              {features.map((feature, i) => (
                <ScrollReveal key={feature.title} delay={i * 0.1}>
                  <FeatureCard {...feature} />
                </ScrollReveal>
              ))}
            </div>
          </div>
        </section>

        {/* How it Works */}
        <section className={styles.section}>
          <div className={`${styles.sectionFrame} ${styles.sectionFrameGold}`}>
            <ScrollReveal>
              <p className={styles.sectionEyebrow}>execution flow</p>
              <h2 className={styles.sectionTitle}>how it works</h2>
              <p className={styles.sectionSubtitle}>
                kasmos decomposes your feature spec into waves of parallel tasks,
                each executed by a specialized agent in its own isolated worktree.
              </p>
            </ScrollReveal>

            <ScrollReveal>
              <LifecycleDiagram />
            </ScrollReveal>

            <div className={styles.agentsGrid}>
              {agents.map((agent, i) => (
                <ScrollReveal key={agent.role} delay={i * 0.08}>
                  <div
                    className={`${styles.agentCard} ${agentToneClasses[agent.color]}`}
                  >
                    <span className={styles.agentIcon}>{agent.icon}</span>
                    <span className={styles.agentRole}>{agent.role}</span>
                    <p className={styles.agentDesc}>{agent.desc}</p>
                  </div>
                </ScrollReveal>
              ))}
            </div>
          </div>
        </section>

        {/* Installation */}
        <section id="install" className={styles.section}>
          <div className={`${styles.sectionFrame} ${styles.sectionFrameFoam}`}>
            <ScrollReveal className={styles.installSection}>
              <p className={styles.sectionEyebrow}>install</p>
              <h2 className={styles.sectionTitle}>get started</h2>
              <p className={styles.sectionSubtitle}>
                install kasmos in seconds. works on macOS, Linux, and Windows.
              </p>
              <InstallTabs />
              <p className={styles.installPrereqs}>
                prerequisites: tmux, gh, and at least one supported ai cli —{" "}
                <a href="/docs/getting-started/prerequisites/">see full list</a>
              </p>
            </ScrollReveal>
          </div>
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
