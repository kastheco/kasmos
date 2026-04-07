"use client";

import { useEffect, useState } from "react";
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
      <a href="/" className={styles.logoWrapper}>
        <img
          src="/logo-k.png"
          alt="kasmos"
          className={styles.logoImage}
          width={32}
          height={32}
        />
      </a>
      <nav className={styles.nav}>
        <a
          href="https://github.com/kastheco/kasmos"
          target="_blank"
          rel="noopener noreferrer"
          className={styles.navLink}
        >
          github
        </a>
        <a
          href="/docs"
          className={styles.navLink}
        >
          docs
        </a>
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
