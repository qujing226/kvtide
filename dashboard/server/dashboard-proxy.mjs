import { request as httpRequest } from "node:http";
import { request as httpsRequest } from "node:https";

const getExecutorsPath = "/kvtide.v1.AdminService/GetExecutors";
const metricsPath = "/api/metrics";
const requestHeaders = ["content-type", "accept", "connect-protocol-version"];
const responseHeaders = [
  "content-type",
  "connect-content-encoding",
  "grpc-status",
  "grpc-message",
];

function selectHeaders(source, names) {
  const selected = {};
  for (const name of names) {
    const value = source[name];
    if (value !== undefined) selected[name] = value;
  }
  return selected;
}

function sendJson(response, statusCode, message, headers = {}) {
  response.writeHead(statusCode, {
    "content-type": "application/json; charset=utf-8",
    ...headers,
  });
  response.end(JSON.stringify({ error: message }));
}

function readBody(request, maxBodyBytes) {
  return new Promise((resolve) => {
    const chunks = [];
    let bytes = 0;
    request.on("data", (chunk) => {
      bytes += chunk.length;
      if (bytes <= maxBodyBytes) chunks.push(chunk);
    });
    request.on("end", () =>
      resolve(bytes > maxBodyBytes ? null : Buffer.concat(chunks)),
    );
    request.on("aborted", () => resolve(null));
  });
}

export function createDashboardProxy({
  adminUpstream,
  maxBodyBytes = 65_536,
  requestTimeoutMs = 10_000,
}) {
  const upstream = new URL(adminUpstream);
  if (upstream.protocol !== "http:" && upstream.protocol !== "https:") {
    throw new TypeError("adminUpstream must use HTTP or HTTPS");
  }

  function forward(request, response, path, method, body) {
    const requestFor = upstream.protocol === "https:" ? httpsRequest : httpRequest;
    const upstreamRequest = requestFor({
      protocol: upstream.protocol,
      hostname: upstream.hostname,
      port: upstream.port,
      path,
      method,
      headers: selectHeaders(request.headers, requestHeaders),
    });
    const timeout = setTimeout(() => {
      upstreamRequest.destroy();
      if (!response.headersSent) sendJson(response, 504, "Upstream request timed out");
    }, requestTimeoutMs);
    timeout.unref?.();

    upstreamRequest.on("response", (upstreamResponse) => {
      clearTimeout(timeout);
      response.writeHead(
        upstreamResponse.statusCode ?? 502,
        selectHeaders(upstreamResponse.headers, responseHeaders),
      );
      upstreamResponse.pipe(response);
    });
    upstreamRequest.on("error", () => {
      clearTimeout(timeout);
      if (!response.headersSent) sendJson(response, 502, "Upstream connection failed");
    });
    upstreamRequest.end(body ?? undefined);
  }

  return async function proxyDashboardRequest(request, response) {
    const pathname = new URL(request.url ?? "/", "http://localhost").pathname;
    if (pathname !== metricsPath && pathname !== getExecutorsPath) return false;

    const expectedMethod = pathname === metricsPath ? "GET" : "POST";
    if (request.method !== expectedMethod) {
      sendJson(response, 405, "Method not allowed", { allow: expectedMethod });
      return true;
    }

    if (pathname === metricsPath) {
      forward(request, response, "/metrics", "GET", null);
      return true;
    }

    const body = await readBody(request, maxBodyBytes);
    if (body === null) {
      sendJson(response, 413, "Request body too large");
      return true;
    }
    forward(request, response, getExecutorsPath, "POST", body);
    return true;
  };
}
