/**
 * Jest runs screens and hooks in React Native's environment (jest-expo).
 * Coverage thresholds come from the code standards (spec §7.1).
 */
module.exports = {
  preset: "jest-expo",
  // Tests live under src/: every file in app/ becomes a route.
  roots: ["<rootDir>/src"],
  setupFiles: ["<rootDir>/jest.setup.js"],
  // packages/crypto imports "./x.js" (NodeNext); Jest must load x.ts.
  moduleNameMapper: { "^(\\.{1,2}/.*)\\.js$": "$1" },
  // The crypto libraries ship ES modules only, so Babel transforms them
  // like React Native's own packages (pnpm keeps them under .pnpm).
  transformIgnorePatterns: [
    "node_modules/(?!(\\.pnpm|((jest-)?react-native|@react-native(-community)?)|expo(nent)?|@expo(nent)?/.*|@expo-google-fonts/.*|react-navigation|@react-navigation/.*|@sentry/react-native|native-base|react-native-svg|ts-mls|@noble|@hpke|@scure|libsodium-wrappers-sumo|libsodium-sumo))",
  ],
  // Test helpers and the root layout (only providers) are not app logic.
  collectCoverageFrom: [
    "src/**/*.{ts,tsx}",
    "app/**/*.{ts,tsx}",
    "!**/*.test.{ts,tsx}",
    "!src/testing/**",
    "!app/_layout.tsx",
  ],
  // Real MLS and Argon2 work runs in several suites at once; on a busy CI
  // runner even a screen test can take longer than Jest's 5 s default.
  testTimeout: 30_000,
  coverageThreshold: { global: { lines: 85, statements: 85, functions: 85, branches: 75 } },
};
