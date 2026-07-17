import { request as httpRequest } from "node:http";
import { request as httpsRequest } from "node:https";

const generatePath = "/kvtide.v1.InferenceService/GenerateStream";
const getRuntimesPath = "/kvtide.v1.AdminService/GetRuntimes";
const metricsPath = "/api/metrics";
const requestHeaders = [
  "content-type",
  "accept",
  "connect-protocol-version",
  "content-encoding",
];
const responseHeaders = [
  "content-type",
  "connect-content-encoding",
  "connect-accept-encoding",
  "grpc-status",
  "grpc-message",
  "grpc-status-details-bin",
];

function parseUpstream(value, name) {
  const upstream = new URL(value);
  if (upstream.protocol !== "http:" && upstream.protocol !== "https:") {
    throw new TypeError(`${name} must use HTTP or HTTPS`);
  }
  return upstream;
}

function selectHeaders(source, names) {
  const selected = {};
  for (const name of names) {
    const value = source[name];
    if (value !== undefined) selected[name] = value;
  }
  return selected;
}

function sendJson(response, statusCode, message, headers = {}) {
  if (response.headersSent || response.destroyed) return;
  response.writeHead(statusCode, {
    "content-type": "application/json; charset=utf-8",
    ...headers,
  });
  response.end(JSON.stringify({ error: message }));
}

function requestFor(upstream) {
  return upstream.protocol === "https:" ? httpsRequest : httpRequest;
}

function readRequestBody(request, maxBodyBytes) {
  return new Promise((resolve) => {
    const chunks = [];
    let receivedBytes = 0;
    let settled = false;

    const cleanup = () => {
      request.off("data", onData);
      request.off("end", onEnd);
      request.off("aborted", onAborted);
      request.off("error", onAborted);
    };
    const settle = (result) => {
      if (settled) return;
      settled = true;
      cleanup();
      resolve(result);
    };
    const onData = (chunk) => {
      receivedBytes += chunk.length;
      if (receivedBytes > maxBodyBytes) {
        request.resume();
        settle({ oversized: true, body: null });
        return;
      }
      chunks.push(chunk);
    };
    const onEnd = () => settle({ oversized: false, body: Buffer.concat(chunks) });
    const onAborted = () => settle({ aborted: true, oversized: false, body: null });

    request.on("data", onData);
    request.once("end", onEnd);
    request.once("aborted", onAborted);
    request.once("error", onAborted);
  });
}

export function createRuntimeProxy({
  apiUpstream,
  metricsUpstream,
  maxBodyBytes = 65_536,
  maxConcurrent = 2,
  requestTimeoutMs = 35_000,
  requestsPerMinute = 10,
  now = Date.now,
}) {
  const api = parseUpstream(apiUpstream, "apiUpstream");
  const metrics = parseUpstream(metricsUpstream, "metricsUpstream");
  const rateWindows = new Map();
  let activeInference = 0;

  function consumeRateLimit(address) {
    const currentTime = now();
    for (const [key, window] of rateWindows) {
      if (currentTime - window.startedAt >= 60_000) rateWindows.delete(key);
    }

    const key = address ?? "unknown";
    let window = rateWindows.get(key);
    if (!window) {
      window = { startedAt: currentTime, count: 0 };
      rateWindows.set(key, window);
    }
    if (window.count >= requestsPerMinute) {
      return Math.max(0, Math.ceil((window.startedAt + 60_000 - currentTime) / 1_000));
    }
    window.count += 1;
    return null;
  }

  function forward(request, response, {
    upstream,
    path,
    method,
    body,
    release = () => {},
  }) {
    let upstreamResponse;
    let timedOut = false;
    let settled = false;

    const finish = () => {
      if (settled) return;
      settled = true;
      release();
    };

    const upstreamRequest = requestFor(upstream)({
      protocol: upstream.protocol,
      hostname: upstream.hostname,
      port: upstream.port,
      method,
      path,
      headers: selectHeaders(request.headers, requestHeaders),
    });

    const timeout = setTimeout(() => {
      timedOut = true;
      upstreamRequest.destroy();
      upstreamResponse?.destroy();
      sendJson(response, 504, "Upstream request timed out");
      finish();
    }, requestTimeoutMs);
    timeout.unref?.();

    const destroyUpstream = () => {
      clearTimeout(timeout);
      upstreamRequest.destroy();
      upstreamResponse?.destroy();
      finish();
    };

    request.once("aborted", destroyUpstream);
    response.once("close", () => {
      if (!response.writableEnded) destroyUpstream();
      finish();
    });
    response.once("finish", finish);

    upstreamRequest.once("response", (receivedResponse) => {
      upstreamResponse = receivedResponse;
      clearTimeout(timeout);
      response.writeHead(
        receivedResponse.statusCode ?? 502,
        selectHeaders(receivedResponse.headers, responseHeaders),
      );
      receivedResponse.once("error", () => {
        if (!response.headersSent) {
          sendJson(response, 502, "Upstream response failed");
        } else {
          response.destroy();
        }
        finish();
      });
      receivedResponse.once("end", finish);
      receivedResponse.pipe(response);
    });

    upstreamRequest.once("error", () => {
      clearTimeout(timeout);
      if (!timedOut && !request.aborted) {
        sendJson(response, 502, "Upstream connection failed");
      }
      finish();
    });

    if (body === null) {
      upstreamRequest.end();
      return;
    }
    upstreamRequest.end(body);
  }

  return async function proxyRuntimeRequest(request, response) {
    const pathname = new URL(request.url ?? "/", "http://localhost").pathname;
    const isInference = pathname === generatePath;
    const isRuntimeInventory = pathname === getRuntimesPath;
    const isMetrics = pathname === metricsPath;
    if (!isInference && !isRuntimeInventory && !isMetrics) return false;

    const expectedMethod = isMetrics ? "GET" : "POST";
    if (request.method !== expectedMethod) {
      sendJson(response, 405, "Method not allowed", { allow: expectedMethod });
      return true;
    }

    if (isMetrics) {
      forward(request, response, {
        upstream: metrics,
        path: "/metrics",
        method: "GET",
        body: null,
      });
      return true;
    }

    const contentLength = Number(request.headers["content-length"]);
    if (
      request.headers["content-length"] !== undefined &&
      Number.isFinite(contentLength) &&
      contentLength > maxBodyBytes
    ) {
      request.resume();
      sendJson(response, 413, "Request body too large");
      return true;
    }

    if (isRuntimeInventory) {
      const body = await readRequestBody(request, maxBodyBytes);
      if (body.aborted) return true;
      if (body.oversized) {
        sendJson(response, 413, "Request body too large");
        return true;
      }
      forward(request, response, {
        upstream: metrics,
        path: getRuntimesPath,
        method: "POST",
        body: body.body,
      });
      return true;
    }

    if (activeInference >= maxConcurrent) {
      request.resume();
      sendJson(response, 429, "Too many concurrent requests");
      return true;
    }

    const retryAfter = consumeRateLimit(request.socket?.remoteAddress);
    if (retryAfter !== null) {
      request.resume();
      sendJson(response, 429, "Rate limit exceeded", {
        "retry-after": String(retryAfter),
      });
      return true;
    }

    activeInference += 1;
    let released = false;
    const release = () => {
      if (released) return;
      released = true;
      activeInference -= 1;
    };
    const body = await readRequestBody(request, maxBodyBytes);
    if (body.aborted) {
      release();
      return true;
    }
    if (body.oversized) {
      sendJson(response, 413, "Request body too large");
      release();
      return true;
    }
    forward(request, response, {
      upstream: api,
      path: generatePath,
      method: "POST",
      body: body.body,
      release,
    });
    return true;
  };
}
