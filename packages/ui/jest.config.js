/**
 * Jest runs the components in React Native's environment (jest-expo).
 * Reanimated is replaced by its official mock (jest.setup.js), since the
 * native animation runtime does not exist in Jest.
 * Coverage thresholds come from the code standards (spec §7.1).
 */
module.exports = {
  preset: "jest-expo",
  roots: ["<rootDir>/src"],
  setupFiles: ["<rootDir>/jest.setup.js"],
  collectCoverageFrom: ["src/**/*.{ts,tsx}", "!src/**/*.test.{ts,tsx}", "!src/index.ts"],
  coverageThreshold: { global: { lines: 85, statements: 85, functions: 85, branches: 75 } },
};
