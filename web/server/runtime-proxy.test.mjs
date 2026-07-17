// @vitest-environment node

import { once } from "node:events";
import { mkdtemp, rm, writeFile } from "node:fs/promises";
import { createServer, request as httpRequest } from "node:http";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { afterEach, describe, expect, it } from "vitest";

import { createRuntimeProxy } from "./runtime-proxy.mjs";
import { createWebServer } from "../server.mjs";

const generatePath = "/kvtide.v1.InferenceService/GenerateStream";
const servers = new Set();
const temporaryDirectories = new Set();

async function listen(handler) {
  const server = createServer(handler);
  servers.add(server);
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  const address = server.address();
  return { server, origin: `http://127.0.0.1:${address.port}` };
}

async function listenServer(server) {
  servers.add(server);
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  const address = server.address();
  return `http://127.0.0.1:${address.port}`;
}

function collectResponse(origin, options = {}) {
  const { body, ...requestOptions } = options;
  return new Promise((resolve, reject) => {
    const request = httpRequest(origin, requestOptions, (response) => {
      const chunks = [];
      response.on("data", (chunk) => chunks.push(chunk));
      response.on("end", () => {
        resolve({
          statusCode: response.statusCode,
          headers: response.headers,
          body: Buffer.concat(chunks).toString(),
        });
      });
    });
    request.on("error", reject);
    if (body !== undefined) request.write(body);
    request.end();
  });
}

async function listenProxy(options) {
  const proxy = createRuntimeProxy(options);
  return listen(async (request, response) => {
    if (!(await proxy(request, response))) {
      response.writeHead(404);
      response.end("static fallback");
    }
  });
}

afterEach(async () => {
  await Promise.all(
    [...servers].map(
      (server) =>
        new Promise((resolve) => {
          server.closeAllConnections?.();
          server.close(() => resolve());
        }),
    ),
  );
  servers.clear();
  await Promise.all(
    [...temporaryDirectories].map((directory) =>
      rm(directory, { recursive: true, force: true }),
    ),
  );
  temporaryDirectories.clear();
});

describe("createRuntimeProxy", () => {
  it("forwards GenerateStream response chunks without buffering", async () => {
    const api = await listen((_request, response) => {
      response.writeHead(200, { "content-type": "application/connect+json" });
      response.write("first");
      setTimeout(() => response.end("second"), 30);
    });
    const metrics = await listen((_request, response) => response.end());
    const proxy = await listenProxy({
      apiUpstream: api.origin,
      metricsUpstream: metrics.origin,
    });

    const events = [];
    await new Promise((resolve, reject) => {
      const request = httpRequest(
        proxy.origin,
        { method: "POST", path: generatePath },
        (response) => {
          response.on("data", (chunk) => events.push(chunk.toString()));
          response.on("end", () => {
            events.push("end");
            resolve();
          });
        },
      );
      request.on("error", reject);
      request.end("request");
    });

    expect(events).toEqual(["first", "second", "end"]);
  });

  it("forwards request data and Connect response metadata", async () => {
    let received;
    const api = await listen((request, response) => {
      const chunks = [];
      request.on("data", (chunk) => chunks.push(chunk));
      request.on("end", () => {
        received = {
          method: request.method,
          url: request.url,
          headers: request.headers,
          body: Buffer.concat(chunks).toString(),
        };
        response.writeHead(206, {
          "content-type": "application/connect+proto",
          "connect-content-encoding": "gzip",
          "connect-accept-encoding": "gzip,identity",
          "grpc-status": "0",
          "grpc-status-details-bin": "details",
          connection: "close",
        });
        response.end("response");
      });
    });
    const metrics = await listen((_request, response) => response.end());
    const proxy = await listenProxy({
      apiUpstream: api.origin,
      metricsUpstream: metrics.origin,
    });

    const result = await collectResponse(proxy.origin, {
      method: "POST",
      path: generatePath,
      body: "connect body",
      headers: {
        "content-type": "application/connect+proto",
        accept: "application/connect+proto",
        "connect-protocol-version": "1",
        "content-encoding": "gzip",
        connection: "close",
      },
    });

    expect(received).toMatchObject({
      method: "POST",
      url: generatePath,
      body: "connect body",
      headers: {
        "content-type": "application/connect+proto",
        accept: "application/connect+proto",
        "connect-protocol-version": "1",
        "content-encoding": "gzip",
      },
    });
    expect(received.headers.connection).not.toBe("close");
    expect(result).toMatchObject({ statusCode: 206, body: "response" });
    expect(result.headers).toMatchObject({
      "content-type": "application/connect+proto",
      "connect-content-encoding": "gzip",
      "connect-accept-encoding": "gzip,identity",
      "grpc-status": "0",
      "grpc-status-details-bin": "details",
    });
  });

  it("maps GET /api/metrics to the metrics upstream /metrics", async () => {
    let received;
    const api = await listen((_request, response) => response.end());
    const metrics = await listen((request, response) => {
      received = { method: request.method, url: request.url };
      response.writeHead(200, { "content-type": "text/plain; version=0.0.4" });
      response.end("metric_total 1\n");
    });
    const proxy = await listenProxy({
      apiUpstream: api.origin,
      metricsUpstream: metrics.origin,
    });

    const result = await collectResponse(proxy.origin, {
      method: "GET",
      path: "/api/metrics",
    });

    expect(received).toEqual({ method: "GET", url: "/metrics" });
    expect(result).toMatchObject({ statusCode: 200, body: "metric_total 1\n" });
    expect(result.headers["content-type"]).toBe("text/plain; version=0.0.4");
  });

  it("returns false outside the public runtime boundary", async () => {
    const proxy = createRuntimeProxy({
      apiUpstream: "http://127.0.0.1:8800",
      metricsUpstream: "http://127.0.0.1:8801",
    });

    await expect(proxy({ method: "GET", url: "/docs" }, {})).resolves.toBe(false);
    await expect(
      proxy(
        { method: "POST", url: "/kvtide.v1.ExecutorService/Execute" },
        {},
      ),
    ).resolves.toBe(false);
    await expect(
      proxy(
        { method: "POST", url: "/kvtide.v1.InferenceService/Generate" },
        {},
      ),
    ).resolves.toBe(false);
  });

  it("returns 405 for a known runtime path with the wrong method", async () => {
    const api = await listen((_request, response) => response.end());
    const metrics = await listen((_request, response) => response.end());
    const proxy = await listenProxy({
      apiUpstream: api.origin,
      metricsUpstream: metrics.origin,
    });

    const result = await collectResponse(proxy.origin, {
      method: "GET",
      path: generatePath,
    });

    expect(result.statusCode).toBe(405);
    expect(result.headers.allow).toBe("POST");
  });

  it("rejects Content-Length 65537 without contacting upstream", async () => {
    let upstreamRequests = 0;
    const api = await listen((_request, response) => {
      upstreamRequests += 1;
      response.end();
    });
    const metrics = await listen((_request, response) => response.end());
    const proxy = await listenProxy({
      apiUpstream: api.origin,
      metricsUpstream: metrics.origin,
    });

    const result = await collectResponse(proxy.origin, {
      method: "POST",
      path: generatePath,
      headers: { "content-length": "65537" },
    });

    expect(result.statusCode).toBe(413);
    expect(result.headers["content-type"]).toMatch(/^application\/json/);
    expect(upstreamRequests).toBe(0);
  });

  it("terminates upstream when a chunked body crosses the limit", async () => {
    let upstreamTerminated;
    const terminated = new Promise((resolve) => {
      upstreamTerminated = resolve;
    });
    const api = await listen((request) => {
      request.on("close", upstreamTerminated);
    });
    const metrics = await listen((_request, response) => response.end());
    const proxy = await listenProxy({
      apiUpstream: api.origin,
      metricsUpstream: metrics.origin,
      maxBodyBytes: 4,
    });

    const resultPromise = new Promise((resolve, reject) => {
      const request = httpRequest(
        proxy.origin,
        { method: "POST", path: generatePath },
        (response) => {
          const chunks = [];
          response.on("data", (chunk) => chunks.push(chunk));
          response.on("end", () =>
            resolve({ statusCode: response.statusCode, body: Buffer.concat(chunks) }),
          );
        },
      );
      request.on("error", reject);
      request.write("1234");
      setTimeout(() => request.end("5"), 15);
    });

    const result = await resultPromise;
    await terminated;
    expect(result.statusCode).toBe(413);
  });

  it("limits concurrency and releases the slot after completion", async () => {
    let releaseFirst;
    const firstHeld = new Promise((resolve) => {
      releaseFirst = resolve;
    });
    let firstArrived;
    const arrived = new Promise((resolve) => {
      firstArrived = resolve;
    });
    let requestCount = 0;
    const api = await listen(async (_request, response) => {
      requestCount += 1;
      if (requestCount === 1) {
        firstArrived();
        await firstHeld;
      }
      response.end(`request-${requestCount}`);
    });
    const metrics = await listen((_request, response) => response.end());
    const proxy = await listenProxy({
      apiUpstream: api.origin,
      metricsUpstream: metrics.origin,
      maxConcurrent: 1,
      requestsPerMinute: 100,
    });

    const first = collectResponse(proxy.origin, { method: "POST", path: generatePath });
    await arrived;
    const second = await collectResponse(proxy.origin, {
      method: "POST",
      path: generatePath,
    });
    expect(second.statusCode).toBe(429);

    releaseFirst();
    expect((await first).statusCode).toBe(200);
    const third = await collectResponse(proxy.origin, {
      method: "POST",
      path: generatePath,
    });
    expect(third.statusCode).toBe(200);
    expect(requestCount).toBe(2);
  });

  it("limits by socket address, ignores XFF, and resets the window", async () => {
    let currentTime = 1_000;
    const api = await listen((_request, response) => response.end("ok"));
    const metrics = await listen((_request, response) => response.end());
    const proxy = await listenProxy({
      apiUpstream: api.origin,
      metricsUpstream: metrics.origin,
      requestsPerMinute: 1,
      now: () => currentTime,
    });

    expect(
      (await collectResponse(proxy.origin, {
        method: "POST",
        path: generatePath,
        headers: { "x-forwarded-for": "198.51.100.1" },
      })).statusCode,
    ).toBe(200);
    const limited = await collectResponse(proxy.origin, {
      method: "POST",
      path: generatePath,
      headers: { "x-forwarded-for": "203.0.113.8" },
    });
    expect(limited.statusCode).toBe(429);
    expect(limited.headers["retry-after"]).toBe("60");

    currentTime += 60_000;
    expect(
      (await collectResponse(proxy.origin, {
        method: "POST",
        path: generatePath,
      })).statusCode,
    ).toBe(200);
  });

  it("returns 502 JSON when the upstream connection fails", async () => {
    const unavailable = await listen((_request, response) => response.end());
    const unavailableOrigin = unavailable.origin;
    await new Promise((resolve) => unavailable.server.close(resolve));
    servers.delete(unavailable.server);
    const metrics = await listen((_request, response) => response.end());
    const proxy = await listenProxy({
      apiUpstream: unavailableOrigin,
      metricsUpstream: metrics.origin,
    });

    const result = await collectResponse(proxy.origin, {
      method: "POST",
      path: generatePath,
    });

    expect(result.statusCode).toBe(502);
    expect(result.headers["content-type"]).toMatch(/^application\/json/);
    expect(result.body).not.toContain(unavailableOrigin);
  });

  it("returns 504 JSON when upstream hangs", async () => {
    const api = await listen(() => {});
    const metrics = await listen((_request, response) => response.end());
    const proxy = await listenProxy({
      apiUpstream: api.origin,
      metricsUpstream: metrics.origin,
      requestTimeoutMs: 25,
    });

    const result = await collectResponse(proxy.origin, {
      method: "POST",
      path: generatePath,
    });

    expect(result.statusCode).toBe(504);
    expect(result.headers["content-type"]).toMatch(/^application\/json/);
  });

  it("releases concurrency when the client aborts", async () => {
    let requestCount = 0;
    let firstArrived;
    const arrived = new Promise((resolve) => {
      firstArrived = resolve;
    });
    const api = await listen((_request, response) => {
      requestCount += 1;
      if (requestCount === 1) {
        firstArrived();
        return;
      }
      response.end("next");
    });
    const metrics = await listen((_request, response) => response.end());
    const proxy = await listenProxy({
      apiUpstream: api.origin,
      metricsUpstream: metrics.origin,
      maxConcurrent: 1,
      requestsPerMinute: 100,
    });

    const aborted = httpRequest(proxy.origin, { method: "POST", path: generatePath });
    aborted.on("error", () => {});
    aborted.end("partial");
    await arrived;
    aborted.destroy();
    await new Promise((resolve) => setTimeout(resolve, 20));

    const next = await collectResponse(proxy.origin, {
      method: "POST",
      path: generatePath,
    });
    expect(next.statusCode).toBe(200);
    expect(next.body).toBe("next");
  });

  it("rejects malformed upstream configuration at creation", () => {
    expect(() =>
      createRuntimeProxy({
        apiUpstream: "not a URL",
        metricsUpstream: "http://127.0.0.1:8801",
      }),
    ).toThrow();
  });
});

describe("createWebServer", () => {
  it("serves SPA history fallback without listening when imported", async () => {
    const staticRoot = await mkdtemp(join(tmpdir(), "kvtide-web-"));
    temporaryDirectories.add(staticRoot);
    await writeFile(join(staticRoot, "index.html"), "<main>SPA fallback</main>");
    const server = createWebServer({ staticRoot, runtimeProxy: async () => false });
    expect(server.listening).toBe(false);

    const origin = await listenServer(server);
    const result = await collectResponse(origin, { method: "GET", path: "/demo" });

    expect(result.statusCode).toBe(200);
    expect(result.body).toBe("<main>SPA fallback</main>");
  });

  it("never sends executor or admin RPC paths to the API upstream", async () => {
    let apiRequests = 0;
    const api = await listen((_request, response) => {
      apiRequests += 1;
      response.end();
    });
    const metrics = await listen((_request, response) => response.end());
    const staticRoot = await mkdtemp(join(tmpdir(), "kvtide-web-"));
    temporaryDirectories.add(staticRoot);
    await writeFile(join(staticRoot, "index.html"), "SPA");
    const runtimeProxy = createRuntimeProxy({
      apiUpstream: api.origin,
      metricsUpstream: metrics.origin,
    });
    const server = createWebServer({ staticRoot, runtimeProxy });
    const origin = await listenServer(server);

    const executor = await collectResponse(origin, {
      method: "POST",
      path: "/kvtide.v1.ExecutorService/Execute",
    });
    const admin = await collectResponse(origin, {
      method: "POST",
      path: "/kvtide.v1.AdminService/Shutdown",
    });

    expect([executor.statusCode, admin.statusCode]).toEqual([200, 200]);
    expect(apiRequests).toBe(0);
  });
});
