/**
 * Jest runs screens and hooks in React Native's environment (jest-expo).
 * Coverage thresholds come from the code standards (spec §7.1).
 */
module.exports = {
  preset: "jest-expo",
  // Tests live under src/: every file in app/ becomes a route.
  roots: ["<rootDir>/src"],
  setupFiles: ["<rootDir>/jest.setup.js"],
  // Test helpers and the root layout (only providers) are not app logic.
  collectCoverageFrom: [
    "src/**/*.{ts,tsx}",
    "app/**/*.{ts,tsx}",
    "!**/*.test.{ts,tsx}",
    "!src/testing/**",
    "!app/_layout.tsx",
  ],
  coverageThreshold: { global: { lines: 85, statements: 85, functions: 85, branches: 75 } },
};
