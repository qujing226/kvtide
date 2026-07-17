import type { ReactNode } from "react";
import { NavLink } from "react-router";

const docsUrl = "https://github.com/qujing226/kvtide#readme";
const githubUrl = "https://github.com/qujing226/kvtide";

type SiteLayoutProps = {
  children: ReactNode;
};

export function SiteLayout({ children }: SiteLayoutProps) {
  return (
    <div className="site-shell">
      <header className="site-header">
        <NavLink className="site-brand" to="/" end>
          KVTide
        </NavLink>
        <nav className="site-nav" aria-label="Primary navigation">
          <a href={docsUrl} target="_blank" rel="noreferrer">
            Docs
          </a>
          <NavLink to="/demo">Demo</NavLink>
          <NavLink to="/lab">Lab</NavLink>
          <NavLink to="/blog">Blog</NavLink>
          <a href={githubUrl} target="_blank" rel="noreferrer">
            GitHub
          </a>
        </nav>
      </header>

      <main className="site-main">{children}</main>

      <footer className="site-footer">
        <strong>KVTide</strong>
        <nav aria-label="Footer navigation">
          <a href={docsUrl}>Docs</a>
          <a href={githubUrl}>GitHub</a>
        </nav>
        <small>Copyright 2026 KVTide. MIT License.</small>
      </footer>
    </div>
  );
}
