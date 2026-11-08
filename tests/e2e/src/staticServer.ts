/**
 * Serves the client's web build for the visual checks, which answer every
 * API request themselves, so no instance is needed. Like the server, it
 * gives index.html for app routes.
 *
 * Usage: node src/staticServer.ts WEB_BUILD_DIRECTORY PORT
 */
import { createReadStream, statSync } from "node:fs";
import { createServer } from "node:http";
import path from "node:path";
import { argv } from "node:process";

const root = path.resolve(argv[2] ?? "../../apps/client/dist");
const port = Number(argv[3] ?? "4174");
const types: Record<string, string> = {
  ".html": "text/html; charset=utf-8",
  ".js": "text/javascript; charset=utf-8",
  ".css": "text/css",
  ".json": "application/json",
  ".png": "image/png",
  ".ttf": "font/ttf",
  ".ico": "image/x-icon",
};

function fileFor(urlPath: string): string {
  const candidate = path.join(root, path.normalize(decodeURIComponent(urlPath)));
  try {
    if (candidate.startsWith(root + path.sep) && statSync(candidate).isFile()) {
      return candidate;
    }
  } catch {
    // Not a file: an app route.
  }
  return path.join(root, "index.html");
}

createServer((request, response) => {
  const file = fileFor(new URL(request.url ?? "/", "http://localhost").pathname);
  response.setHeader("Content-Type", types[path.extname(file)] ?? "application/octet-stream");
  createReadStream(file)
    .on("error", () => {
      response.writeHead(404).end("build the web client first: pnpm --filter @nexus-chat/client build:web");
    })
    .pipe(response);
}).listen(port, "127.0.0.1");
