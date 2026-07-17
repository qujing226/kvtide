import { createReadStream } from "node:fs";
import { stat } from "node:fs/promises";
import { createServer } from "node:http";
import { extname, join, resolve, sep } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

import { createRuntimeProxy } from "./server/runtime-proxy.mjs";

const root = fileURLToPath(new URL("./dist/", import.meta.url));
const contentTypes = {
  ".css": "text/css; charset=utf-8",
  ".html": "text/html; charset=utf-8",
  ".ico": "image/x-icon",
  ".js": "text/javascript; charset=utf-8",
  ".json": "application/json; charset=utf-8",
  ".map": "application/json; charset=utf-8",
  ".svg": "image/svg+xml",
  ".woff": "font/woff",
  ".woff2": "font/woff2",
};

export function createWebServer({
  runtimeProxy = async () => false,
  staticRoot = root,
} = {}) {
  const resolvedRoot = resolve(staticRoot);
  return createServer(async (request, response) => {
    if (await runtimeProxy(request, response)) return;

    let pathname;
    try {
      pathname = decodeURIComponent(
        new URL(request.url ?? "/", "http://localhost").pathname,
      );
    } catch {
      response.writeHead(400);
      response.end("Bad request");
      return;
    }

    let filePath = resolve(resolvedRoot, `.${pathname}`);
    if (
      filePath !== resolvedRoot &&
      !filePath.startsWith(`${resolvedRoot}${sep}`)
    ) {
      filePath = join(resolvedRoot, "index.html");
    }

    try {
      const fileStat = await stat(filePath);
      if (fileStat.isDirectory()) {
        filePath = join(filePath, "index.html");
      }
    } catch {
      filePath = join(resolvedRoot, "index.html");
    }

    response.setHeader(
      "Content-Type",
      contentTypes[extname(filePath)] ?? "application/octet-stream",
    );
    createReadStream(filePath)
      .on("error", () => {
        if (!response.headersSent) response.writeHead(404);
        response.end("Not found");
      })
      .pipe(response);
  });
}

const entryPath = process.argv[1] ? pathToFileURL(resolve(process.argv[1])).href : "";
if (import.meta.url === entryPath) {
  const port = Number(process.env.PORT ?? 5173);
  const runtimeProxy = createRuntimeProxy({
    apiUpstream: process.env.KVTIDE_API_UPSTREAM ?? "http://server:8800",
    metricsUpstream:
      process.env.KVTIDE_METRICS_UPSTREAM ?? "http://server:8801",
  });
  createWebServer({ runtimeProxy }).listen(port, "0.0.0.0", () => {
    console.log(`web listening on 0.0.0.0:${port}`);
  });
}
