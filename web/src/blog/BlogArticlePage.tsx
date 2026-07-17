import Markdown from "react-markdown";
import { Link, useParams } from "react-router";
import remarkGfm from "remark-gfm";

import { NotFoundPage } from "../site/NotFoundPage";
import { RoutePage } from "../site/RoutePage";
import { blogEntries, type BlogEntry } from "./entries";

type BlogArticlePageProps = {
  focusOnMount: boolean;
  entries?: readonly BlogEntry[];
};

export function BlogArticlePage({
  focusOnMount,
  entries = blogEntries,
}: BlogArticlePageProps) {
  const { slug } = useParams();
  const entry = entries.find((candidate) => candidate.slug === slug);

  if (!entry) {
    return <NotFoundPage focusOnMount={focusOnMount} />;
  }

  return (
    <RoutePage title={entry.title} focusOnMount={focusOnMount}>
      <article className="blog-article">
        <header>
          <time dateTime={entry.publishedAt}>{entry.publishedAt}</time>
          <p>{entry.summary}</p>
        </header>
        <div className="blog-prose">
          <Markdown remarkPlugins={[remarkGfm]}>{entry.content}</Markdown>
        </div>
        <Link className="blog-back" to="/blog">
          Back to Blog
        </Link>
      </article>
    </RoutePage>
  );
}
