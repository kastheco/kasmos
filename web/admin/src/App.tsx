import { BrowserRouter, Routes, Route } from "react-router";
import Layout from "./Layout";
import DashboardPage from "./pages/DashboardPage";
import TasksPage from "./pages/TasksPage";
import TaskDetailPage from "./pages/TaskDetailPage";
import AuditPage from "./pages/AuditPage";
import InstancesPage from "./pages/InstancesPage";
import ConfigPage from "./pages/ConfigPage";
import AccessLoginPreviewPage from "./pages/AccessLoginPreviewPage";
import { ProjectProvider } from "./hooks/useProject";

export default function App() {
  return (
    <BrowserRouter basename="/admin">
      <ProjectProvider>
        <Routes>
          <Route path="/access-login-preview" element={<AccessLoginPreviewPage />} />
          <Route element={<Layout />}>
            <Route path="/" element={<DashboardPage />} />
            <Route path="/tasks" element={<TasksPage />} />
            <Route path="/tasks/:filename" element={<TaskDetailPage />} />
            <Route path="/audit" element={<AuditPage />} />
            <Route path="/instances" element={<InstancesPage />} />
            <Route path="/config" element={<ConfigPage />} />
          </Route>
        </Routes>
      </ProjectProvider>
    </BrowserRouter>
  );
}
