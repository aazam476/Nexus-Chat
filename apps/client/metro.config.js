/* eslint-disable @typescript-eslint/no-require-imports -- Metro loads its configuration as CommonJS. */
/**
 * Metro settings: Expo's defaults, plus two rules.
 *
 * - packages/crypto is written for Node's module resolution and imports its
 *   own files as "./x.js"; Metro looks for x.ts instead when that file
 *   exists.
 * - On iOS and Android, libsodium resolves to its CommonJS build. Hermes has
 *   no WebAssembly, and only that build carries libsodium's plain
 *   JavaScript fallback; the ES module build is WebAssembly only. Browsers
 *   and the desktop app keep the ES module build.
 */
const { getDefaultConfig } = require("expo/metro-config");
const fs = require("node:fs");
const path = require("node:path");

const config = getDefaultConfig(__dirname);
const defaultResolve = config.resolver.resolveRequest;

const commonJsOnNative = new Set(["libsodium-wrappers-sumo", "libsodium-sumo"]);
const isNodeBuiltin = (name) => require("node:module").isBuiltin(name);

config.resolver.resolveRequest = (context, moduleName, platform) => {
  const resolve = defaultResolve ?? context.resolveRequest;
  const native = platform === "android" || platform === "ios";
  if (native && commonJsOnNative.has(moduleName)) {
    // Node's resolution with the "require" condition, from the importing file.
    const paths = [path.dirname(context.originModulePath)];
    return { type: "sourceFile", filePath: require.resolve(moduleName, { paths }) };
  }
  // That build loads Node's built-in modules only when it runs under Node.
  if (
    native &&
    /[\\/]libsodium(-wrappers)?-sumo[\\/]/.test(context.originModulePath) &&
    isNodeBuiltin(moduleName)
  ) {
    return { type: "empty" };
  }
  if (moduleName.startsWith(".") && moduleName.endsWith(".js")) {
    const candidate = path.resolve(
      path.dirname(context.originModulePath),
      moduleName.replace(/\.js$/, ".ts"),
    );
    if (fs.existsSync(candidate)) {
      return { type: "sourceFile", filePath: candidate };
    }
  }
  return resolve(context, moduleName, platform);
};

module.exports = config;
