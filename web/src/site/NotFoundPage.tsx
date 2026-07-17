import { Link } from "react-router";

import { PageTransition } from "./PageTransition";

export function NotFoundPage() {
  return (
    <PageTransition>
      <h1>Page not found</h1>
      <Link to="/">Home</Link>
    </PageTransition>
  );
}
