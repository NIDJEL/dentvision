import type { ReactNode } from "react";
import type { User } from "../api/client";

type AppShellProps = {
  user: User | null;
  title: string;
  actions?: ReactNode;
  children: ReactNode;
  onHome: () => void;
  onLogout: () => void;
};

export function AppShell({
  user,
  title,
  actions,
  children,
  onHome,
  onLogout,
}: AppShellProps) {
  return (
    <div className="app-shell">
      <header className="topbar">
        <button className="brand" type="button" onClick={onHome}>
          DentVision
        </button>
        <div className="topbar-user">
          {user ? <span>{user.full_name || user.email}</span> : null}
          <button className="secondary-button" type="button" onClick={onLogout}>
            Logout
          </button>
        </div>
      </header>
      <main className="page">
        <div className="page-heading">
          <h1>{title}</h1>
          {actions}
        </div>
        {children}
      </main>
    </div>
  );
}
