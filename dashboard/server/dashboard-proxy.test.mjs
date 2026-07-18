// @vitest-environment node

import { once } from "node:events";
import { createServer, request as httpRequest } from "node:http";
import { afterEach, describe, expect, it } from "vitest";

import { createDashboardProxy } from "./dashboard-proxy.mjs";

const getExecutorsPath = "/kvtide.v1.AdminService/GetExecutors";
const servers = new Set();

async function listen(handler) {
  const server = createServer(handler);
  servers.add(server);
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  const address = server.address();
  return `http://127.0.0.1:${address.port}`;
}

function collect(origin, { method = "GET", path = "/", body } = {}) {
  return new Promise((resolve, reject) => {
    const request = httpRequest(origin, { method, path }, (response) => {
      const chunks = [];
      response.on("data", (chunk) => chunks.push(chunk));
      response.on("end", () =>
        resolve({
          statusCode: response.statusCode,
          body: Buffer.concat(chunks).toString(),
        }),
      );
    });
    request.on("error", reject);
    if (body !== undefined) request.write(body);
    request.end();
  });
}

async function listenProxy(upstream) {
  const proxy = createDashboardProxy({ adminUpstream: upstream });
  return listen(async (request, response) => {
    if (!(await proxy(request, response))) {
      response.writeHead(404);
      response.end("static");
    }
  });
}

afterEach(async () => {
  await Promise.all(
    [...servers].map(
      (server) =>
        new Promise((resolve) => {
          server.closeAllConnections?.();
          server.close(resolve);
        }),
    ),
  );
  servers.clear();
});

describe("createDashboardProxy", () => {
  it("maps the dashboard metrics endpoint to the Engine", async () => {
    const upstream = await listen((request, response) => {
      expect(request.url).toBe("/metrics");
      response.end("llm_active_requests 1\n");
    });
    const proxy = await listenProxy(upstream);

    await expect(collect(proxy, { path: "/api/metrics" })).resolves.toMatchObject({
      statusCode: 200,
      body: "llm_active_requests 1\n",
    });
  });

  it("forwards only GetExecutors from the Admin service", async () => {
    const upstream = await listen((request, response) => {
      expect(request.url).toBe(getExecutorsPath);
      expect(request.method).toBe("POST");
      request.resume();
      response.end('{"executors":[]}');
    });
    const proxy = await listenProxy(upstream);

    await expect(
      collect(proxy, { method: "POST", path: getExecutorsPath, body: "{}" }),
    ).resolves.toMatchObject({ statusCode: 200, body: '{"executors":[]}' });
    await expect(
      collect(proxy, {
        method: "POST",
        path: "/kvtide.v1.InferenceService/Generate",
      }),
    ).resolves.toMatchObject({ statusCode: 404 });
  });
});
