import { Link } from "react-router";

import { RoutePage } from "./RoutePage";

type NotFoundPageProps = {
  focusOnMount: boolean;
};

export function NotFoundPage({ focusOnMount }: NotFoundPageProps) {
  return (
    <RoutePage title="Page not found" focusOnMount={focusOnMount}>
      <Link to="/">Home</Link>
    </RoutePage>
  );
}
