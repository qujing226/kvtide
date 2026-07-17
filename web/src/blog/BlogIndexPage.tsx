import { Link } from "react-router";

import { RoutePage } from "../site/RoutePage";
import { blogEntries, type BlogEntry } from "./entries";

type BlogIndexPageProps = {
  focusOnMount: boolean;
  entries?: readonly BlogEntry[];
};

export function BlogIndexPage({
  focusOnMount,
  entries = blogEntries,
}: BlogIndexPageProps) {
  return (
    <RoutePage title="Blog" focusOnMount={focusOnMount}>
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
    </RoutePage>
  );
}
