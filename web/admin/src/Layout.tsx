import { NavLink, Outlet } from "react-router";
import styles from "./Layout.module.css";
import { useProject } from "./hooks/useProject";
import ProjectSwitcher from "./components/ProjectSwitcher";

export default function Layout() {
  const { projectSearch } = useProject();

  return (
    <div className={styles.container}>
      <nav className={styles.sidebar}>
        <div className={styles.logo}>
          <span>kas</span>
          <span className={styles.logoSub}>admin</span>
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
              tasks
            </NavLink>
          </li>
          <li>
            <NavLink
              to={{ pathname: "/audit", search: projectSearch }}
              className={({ isActive }) =>
                `${styles.navLink} ${isActive ? styles.active : ""}`
              }
            >
              audit
            </NavLink>
          </li>
        </ul>
        <div className={styles.switcherSection}>
          <ProjectSwitcher />
        </div>
      </nav>
      <main className={styles.main}>
        <Outlet />
      </main>
    </div>
  );
}
