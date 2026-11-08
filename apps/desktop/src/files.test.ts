import { mkdirSync, mkdtempSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";

import { appOrigin, contentTypeFor, fileFor } from "./files";

describe("fileFor", () => {
  const root = mkdtempSync(path.join(tmpdir(), "nexus-web-"));
  writeFileSync(path.join(root, "index.html"), "<html></html>");
  mkdirSync(path.join(root, "_expo", "static", "js"), { recursive: true });
  writeFileSync(path.join(root, "_expo", "static", "js", "app.js"), "");
  writeFileSync(path.join(path.dirname(root), "outside.txt"), "secret");

  it("serves files of the web build", () => {
    expect(fileFor(`${appOrigin}/_expo/static/js/app.js`, root)).toBe(
      path.join(root, "_expo", "static", "js", "app.js"),
    );
  });

  it.each([
    ["the root", "/"],
    ["an app route", "/login"],
    ["a directory", "/_expo/static"],
    ["an address that climbs out", "/../outside.txt"],
    ["an encoded climb", "/%2e%2e/outside.txt"],
    ["an encoded separator climb", "/..%2foutside.txt"],
  ])("gives index.html for %s", (_name, address) => {
    expect(fileFor(`${appOrigin}${address}`, root)).toBe(path.join(root, "index.html"));
  });
});

describe("contentTypeFor", () => {
  it.each([
    ["index.html", "text/html; charset=utf-8"],
    ["APP.JS", "text/javascript; charset=utf-8"],
    ["logo.svg", "image/svg+xml"],
    ["font.woff2", "font/woff2"],
    ["data.bin", "application/octet-stream"],
  ])("%s is %s", (file, type) => {
    expect(contentTypeFor(file)).toBe(type);
  });
});
