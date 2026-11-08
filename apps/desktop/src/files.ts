/**
 * Mapping `app://nexus-chat/...` addresses to files of the web build. The
 * app is a single-page app, so any address that is not a file gets
 * index.html, exactly as the server serves it.
 */
import { existsSync, statSync } from "node:fs";
import path from "node:path";

/** The origin the web build is served from inside the app. */
export const appOrigin = "app://nexus-chat";

/**
 * The file for `url` inside `webDirectory`, never outside it: an address
 * that climbs out with `..` gets index.html like any unknown path.
 */
export function fileFor(url: string, webDirectory: string): string {
  const root = path.resolve(webDirectory);
  const pathname = decodeURIComponent(new URL(url).pathname);
  const candidate = path.resolve(root, `.${path.posix.normalize(`/${pathname}`)}`);
  const inside = candidate === root || candidate.startsWith(root + path.sep);
  if (inside && existsSync(candidate) && statSync(candidate).isFile()) {
    return candidate;
  }
  return path.join(root, "index.html");
}

/** Content types for the files a web build contains. */
const contentTypes: Record<string, string> = {
  ".html": "text/html; charset=utf-8",
  ".js": "text/javascript; charset=utf-8",
  ".css": "text/css; charset=utf-8",
  ".json": "application/json",
  ".png": "image/png",
  ".svg": "image/svg+xml",
  ".ico": "image/x-icon",
  ".ttf": "font/ttf",
  ".woff2": "font/woff2",
};

/** The content type for a file, by its extension. */
export function contentTypeFor(file: string): string {
  return contentTypes[path.extname(file).toLowerCase()] ?? "application/octet-stream";
}
