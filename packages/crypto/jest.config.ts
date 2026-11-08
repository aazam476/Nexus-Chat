import type { Config } from "jest";

/**
 * Jest settings. Crypto code needs 100% branch coverage (spec §7.1).
 * The interop test vectors are fetched by scripts/fetch-mls-vectors.sh.
 */
const config: Config = {
  preset: "ts-jest/presets/default-esm",
  testEnvironment: "node",
  roots: ["<rootDir>/src"],
  // Source files import "./x.js" (required by NodeNext); Jest must load x.ts.
  moduleNameMapper: { "^(\\.{1,2}/.*)\\.js$": "$1" },
  collectCoverageFrom: ["src/**/*.ts", "!src/**/*.test.ts", "!src/testing/**"],
  coverageThreshold: { global: { lines: 100, statements: 100, functions: 100, branches: 100 } },
  // Large groups and the interop vectors take a while.
  testTimeout: 120_000,
};

export default config;
