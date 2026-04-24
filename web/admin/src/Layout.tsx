import { NavLink, Outlet } from "react-router";
import styles from "./Layout.module.css";
import { useProject } from "./hooks/useProject";
import { ToastProvider } from "./hooks/useToast";
import ProjectSwitcher from "./components/ProjectSwitcher";
import logoFull from "./assets/logo-full.png";
import { BRAND } from "./brand";

export default function Layout() {
  const { projectSearch } = useProject();

  return (
    <ToastProvider>
    <div className={styles.container}>
      <nav className={styles.sidebar}>
        <div className={styles.logo}>
          <img src={logoFull} alt={BRAND.wordmarkAlt} className={styles.logoImg} />
          <span className={`brand-hq-suffix ${styles.hqSuffix}`}>{BRAND.shortName}</span>
        </div>
        <div className={styles.switcherSection}>
          <ProjectSwitcher />
        </div>
        <ul className={styles.navList}>
          <li>
            <NavLink
              to={{ pathname: "/", search: projectSearch }}
              end
              className={({ isActive }) =>
                `${styles.navLink} ${isActive ? styles.active : ""}`
              }
            >
              dashboard
            </NavLink>
          </li>
          <li>
            <NavLink
              to={{ pathname: "/tasks", search: projectSearch }}
              className={({ isActive }) =>
                `${styles.navLink} ${isActive ? styles.active : ""}`
              }
            >
              plans
            </NavLink>
          </li>
          <li>
            <NavLink
              to={{ pathname: "/instances", search: projectSearch }}
              className={({ isActive }) =>
                `${styles.navLink} ${isActive ? styles.active : ""}`
              }
            >
              agents
            </NavLink>
          </li>
          <li>
            <NavLink
              to={{ pathname: "/audit", search: projectSearch }}
              className={({ isActive }) =>
                `${styles.navLink} ${isActive ? styles.active : ""}`
              }
            >
              audit log
            </NavLink>
          </li>
          <li>
            <NavLink
              to={{ pathname: "/config", search: projectSearch }}
              className={({ isActive }) =>
                `${styles.navLink} ${isActive ? styles.active : ""}`
              }
            >
              config
            </NavLink>
          </li>
        </ul>
      </nav>
      <main className={styles.main}>
        <Outlet />
      </main>
    </div>
    </ToastProvider>
  );
}
