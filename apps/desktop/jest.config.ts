import type { Config } from "jest";

/**
 * Jest settings. main.ts and preload.ts only wire Electron's APIs to the
 * functions tested here; they are exercised by starting the app.
 */
const config: Config = {
  preset: "ts-jest",
  testEnvironment: "node",
  roots: ["<rootDir>/src"],
  collectCoverageFrom: ["src/**/*.ts", "!src/main.ts", "!src/preload.ts", "!src/**/*.test.ts"],
  coverageThreshold: { global: { lines: 100, statements: 100, functions: 100, branches: 100 } },
};

export default config;
