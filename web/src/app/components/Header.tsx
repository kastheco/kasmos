"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import styles from "./Header.module.css";

export default function Header() {
  const [scrolled, setScrolled] = useState(false);

  useEffect(() => {
    const onScroll = () => setScrolled(window.scrollY > 50);
    window.addEventListener("scroll", onScroll, { passive: true });
    return () => window.removeEventListener("scroll", onScroll);
  }, []);

  return (
    <header className={`${styles.header} ${scrolled ? styles.scrolled : ""}`}>
      <Link href="/" className={styles.logoWrapper}>
        <img
          src="/logo-k.png"
          alt="kasmos"
          className={`${styles.logoImage} ${styles.logoK} ${scrolled ? styles.logoHidden : ""}`}
        />
        <img
          src="/logo-full.png"
          alt="kasmos"
          className={`${styles.logoImage} ${styles.logoFull} ${scrolled ? "" : styles.logoHidden}`}
        />
      </Link>
      <nav className={styles.nav}>
        <a
          href="https://github.com/kastheco/kasmos"
          target="_blank"
          rel="noopener noreferrer"
          className={styles.navLink}
        >
          github
        </a>
        <Link
          href="/docs"
          className={styles.navLink}
        >
          docs
        </Link>
        <a
          href="https://github.com/kastheco/kasmos/releases"
          target="_blank"
          rel="noopener noreferrer"
          className={styles.navLink}
        >
          releases
        </a>
      </nav>
    </header>
  );
}
