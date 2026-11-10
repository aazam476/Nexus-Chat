/* eslint-disable @typescript-eslint/no-require-imports -- Metro loads its configuration as CommonJS. */
/**
 * Metro settings: Expo's defaults, plus one rule. packages/crypto is
 * written for Node's module resolution and imports its own files as
 * "./x.js"; Metro looks for x.ts instead when that file exists.
 */
const { getDefaultConfig } = require("expo/metro-config");
const fs = require("node:fs");
const path = require("node:path");

const config = getDefaultConfig(__dirname);
const defaultResolve = config.resolver.resolveRequest;

config.resolver.resolveRequest = (context, moduleName, platform) => {
  if (moduleName.startsWith(".") && moduleName.endsWith(".js")) {
    const candidate = path.resolve(
      path.dirname(context.originModulePath),
      moduleName.replace(/\.js$/, ".ts"),
    );
    if (fs.existsSync(candidate)) {
      return { type: "sourceFile", filePath: candidate };
    }
  }
  return (defaultResolve ?? context.resolveRequest)(context, moduleName, platform);
};

module.exports = config;
