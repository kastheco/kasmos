import styles from "./AccessLoginPreviewPage.module.css";
import { BRAND } from "../brand";

export default function AccessLoginPreviewPage() {
  return (
    <div
      className={styles.preview}
      style={{ background: BRAND.cloudflare.backgroundColor }}
    >
      <header className={styles.header}>{BRAND.cloudflare.headerText}</header>
      <main className={styles.card}>
        <img
          src={BRAND.accessLogoPublicPath}
          alt={BRAND.wordmarkAlt}
          className={styles.logo}
        />
        <h1 className={styles.orgName}>{BRAND.cloudflare.organizationName}</h1>
        <p className={styles.hint}>sign in with your identity provider</p>
        <button type="button" className={styles.idpButton} disabled>
          continue with identity provider (preview only)
        </button>
      </main>
      <footer className={styles.footer}>{BRAND.cloudflare.footerText}</footer>
    </div>
  );
}
