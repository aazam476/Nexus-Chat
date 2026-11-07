import type { Config } from "jest";

/**
 * Jest settings. Coverage thresholds come from the code standards; the
 * generated client is excluded because it is tested by its generator.
 */
const config: Config = {
  preset: "ts-jest/presets/default-esm",
  testEnvironment: "node",
  roots: ["<rootDir>/src"],
  // Source files import "./x.js" (required by NodeNext); Jest must load x.ts.
  moduleNameMapper: { "^(\\.{1,2}/.*)\\.js$": "$1" },
  collectCoverageFrom: ["src/**/*.ts", "!src/generated/**", "!src/**/*.test.ts"],
  coverageThreshold: { global: { lines: 85, statements: 85, functions: 85, branches: 75 } },
};

export default config;
