import {
  createContext,
  useContext,
  useEffect,
  type JSX,
} from "react";
import { useSearchParams } from "react-router";
import { listProjects, resolveProjectName } from "../api";
import { useAutoRefresh } from "./useAutoRefresh";

const STORAGE_KEY = "kasmos-admin-project";

export type ProjectContextValue = {
  project: string;
  projects: string[];
  loading: boolean;
  projectSearch: string;
  setProject: (project: string) => void;
};

const ProjectContext = createContext<ProjectContextValue | null>(null);

export function ProjectProvider({
  children,
}: {
  children: React.ReactNode;
}): JSX.Element {
  const [searchParams, setSearchParams] = useSearchParams();
  const { data: fetchedProjects, loading } = useAutoRefresh<string[]>(
    () => listProjects(),
    [],
    30000,
  );

  const projects = fetchedProjects ?? [];

  // Resolve the active project:
  // 1. ?project= param when non-empty and in the fetched list
  // 2. localStorage when in the fetched list
  // 3. first project returned by /v1/projects
  // 4. legacy resolveProjectName fallback when list is empty or unavailable
  const urlProject = searchParams.get("project") ?? "";
  const storedProject = localStorage.getItem(STORAGE_KEY) ?? "";

  let project: string;
  if (projects.length === 0) {
    // Fallback: keep whatever is in the URL or use legacy resolver
    project =
      urlProject || resolveProjectName(window.location.search);
  } else if (urlProject && projects.includes(urlProject)) {
    project = urlProject;
  } else if (storedProject && projects.includes(storedProject)) {
    project = storedProject;
  } else {
    project = projects[0];
  }

  const projectSearch = `?project=${encodeURIComponent(project)}`;

  const setProject = (next: string) => {
    localStorage.setItem(STORAGE_KEY, next);
    setSearchParams(
      (prev) => {
        const p = new URLSearchParams(prev);
        p.set("project", next);
        return p;
      },
      { replace: false },
    );
  };

  // Write the resolved project back into the URL when it differs (replace so
  // we don't pollute history with auto-selected values).  Skip while the
  // project list is still loading — the fallback value resolved from an empty
  // list must not be committed to the URL or it will override localStorage /
  // first-project precedence once the real list arrives.
  useEffect(() => {
    if (loading) return;
    if (project && project !== urlProject) {
      setSearchParams(
        (prev) => {
          const p = new URLSearchParams(prev);
          p.set("project", project);
          return p;
        },
        { replace: true },
      );
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [project, loading]);

  // Guard: if the list refreshed and the current selection disappeared, switch
  // to the first available project.
  useEffect(() => {
    if (projects.length === 0) return;
    if (!projects.includes(project)) {
      const next = projects[0];
      localStorage.setItem(STORAGE_KEY, next);
      setSearchParams(
        (prev) => {
          const p = new URLSearchParams(prev);
          p.set("project", next);
          return p;
        },
        { replace: true },
      );
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [projects]);

  return (
    <ProjectContext.Provider
      value={{ project, projects, loading, projectSearch, setProject }}
    >
      {children}
    </ProjectContext.Provider>
  );
}

export function useProject(): ProjectContextValue {
  const ctx = useContext(ProjectContext);
  if (!ctx) {
    throw new Error("useProject must be used within a ProjectProvider");
  }
  return ctx;
}
