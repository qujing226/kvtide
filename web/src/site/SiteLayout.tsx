import type { ReactNode } from "react";
import { Link, NavLink, useMatch } from "react-router";

const docsUrl = "https://github.com/qujing226/kvtide#readme";
const githubUrl = "https://github.com/qujing226/kvtide";

type SiteLayoutProps = {
  children: ReactNode;
};

export function SiteLayout({ children }: SiteLayoutProps) {
  const blogIndexMatch = useMatch("/blog");
  const blogPostMatch = useMatch("/blog/:slug");
  const blogIsActive = Boolean(blogIndexMatch || blogPostMatch);

  return (
    <div className="site-shell">
      <header className="site-header">
        <NavLink className="site-brand" to="/" end>
          <img src="/favicon.svg" alt="" aria-hidden="true" />
          KVTide
        </NavLink>
        <nav className="site-nav" aria-label="Primary navigation">
          <a className="site-nav-external" href={docsUrl}>Docs</a>
          <NavLink to="/demo" end>
            Demo
          </NavLink>
          <NavLink to="/lab" end>
            Lab
          </NavLink>
          <Link
            className={blogIsActive ? "active" : undefined}
            to="/blog"
            aria-current={blogIsActive ? "page" : undefined}
          >
            Blog
          </Link>
          <a className="site-nav-external" href={githubUrl}>GitHub</a>
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
