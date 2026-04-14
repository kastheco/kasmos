import {
  createContext,
  useCallback,
  useContext,
  useRef,
  useState,
  type JSX,
} from "react";
import Toast from "../components/Toast";

export type ToastKind = "success" | "error";

export interface ToastOptions {
  kind?: ToastKind;
  ttl?: number;
}

export interface ToastItem {
  id: number;
  message: string;
  kind: ToastKind;
}

export interface ToastContextValue {
  show: (message: string, options?: ToastOptions) => void;
}

const ToastContext = createContext<ToastContextValue | null>(null);

const DEFAULT_TTL = 3500;

export function ToastProvider({
  children,
}: {
  children: React.ReactNode;
}): JSX.Element {
  const [toasts, setToasts] = useState<ToastItem[]>([]);
  const counter = useRef(0);

  const dismiss = useCallback((id: number) => {
    setToasts((prev) => prev.filter((t) => t.id !== id));
  }, []);

  const show = useCallback((message: string, options: ToastOptions = {}) => {
    const id = ++counter.current;
    const kind: ToastKind = options.kind ?? "success";
    const ttl = options.ttl ?? DEFAULT_TTL;

    setToasts((prev) => [...prev, { id, message, kind }]);
    setTimeout(() => dismiss(id), ttl);
  }, [dismiss]);

  return (
    <ToastContext.Provider value={{ show }}>
      {children}
      <Toast toasts={toasts} onDismiss={dismiss} />
    </ToastContext.Provider>
  );
}

export function useToast(): ToastContextValue {
  const ctx = useContext(ToastContext);
  if (!ctx) {
    throw new Error("useToast must be used within a ToastProvider");
  }
  return ctx;
}
