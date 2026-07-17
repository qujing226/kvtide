export type BlogEntry = {
  slug: string;
  title: string;
  summary: string;
  publishedAt: string;
  content: string;
};

export const blogEntries: readonly BlogEntry[] = [];
