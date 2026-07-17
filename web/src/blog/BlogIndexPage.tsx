import { useEffect, useRef } from "react";
import { Link } from "react-router";

import { PageTransition } from "../site/PageTransition";
import { blogEntries, type BlogEntry } from "./entries";
import "./blog.css";

type BlogIndexPageProps = {
  focusOnMount: boolean;
  entries?: readonly BlogEntry[];
};

export function BlogIndexPage({
  focusOnMount,
  entries = blogEntries,
}: BlogIndexPageProps) {
  const pageRef = useRef<HTMLElement>(null);

  useEffect(() => {
    document.title = "Blog | KVTide";
    if (focusOnMount) pageRef.current?.focus();
  }, [focusOnMount]);

  return (
    <PageTransition>
      <section className="blog-index" aria-label="Blog" ref={pageRef} tabIndex={-1}>
        {entries.length === 0 ? (
          <div className="blog-empty">
            <span>Design notes</span>
            <p>No entries yet.</p>
          </div>
        ) : (
          <div className="blog-list">
            {entries.map((entry) => (
              <article key={entry.slug}>
                <time dateTime={entry.publishedAt}>{entry.publishedAt}</time>
                <h2>
                  <Link to={`/blog/${entry.slug}`}>{entry.title}</Link>
                </h2>
                <p>{entry.summary}</p>
              </article>
            ))}
          </div>
        )}
      </section>
    </PageTransition>
  );
}
