import type { Config } from "jest";

/**
 * Jest settings for mutation testing (Stryker): jest.config.ts without the
 * interop vectors, which exercise ts-mls rather than our code and would
 * take hours if run for every mutant, and without coverage gates.
 */
const config: Config = {
  preset: "ts-jest/presets/default-esm",
  testEnvironment: "node",
  roots: ["<rootDir>/src"],
  moduleNameMapper: { "^(\\.{1,2}/.*)\\.js$": "$1" },
  testPathIgnorePatterns: ["/node_modules/", "vectors\\.test\\.ts$"],
  testTimeout: 120_000,
};

export default config;
