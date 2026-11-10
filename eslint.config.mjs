// Shared ESLint configuration (code standards §8.3). Every workspace's
// `pnpm lint` uses this file; the build fails on any error.
import eslint from "@eslint/js";
import eslintReact from "@eslint-react/eslint-plugin";
import i18next from "eslint-plugin-i18next";
import jsdoc from "eslint-plugin-jsdoc";
import reactHooks from "eslint-plugin-react-hooks";
import reactNativeA11y from "eslint-plugin-react-native-a11y";
import tsdoc from "eslint-plugin-tsdoc";
import globals from "globals";
import tseslint from "typescript-eslint";

export default tseslint.config(
  {
    ignores: [
      "**/dist/**",
      "**/coverage/**",
      "**/generated/**",
      "**/node_modules/**",
      "**/.stryker-tmp/**",
      "**/reports/**",
    ],
  },
  eslint.configs.recommended,
  ...tseslint.configs.strictTypeChecked,
  ...tseslint.configs.stylisticTypeChecked,
  {
    languageOptions: {
      parserOptions: { projectService: true, tsconfigRootDir: import.meta.dirname },
      globals: { ...globals.node },
    },
    plugins: { jsdoc, tsdoc },
    rules: {
      "@typescript-eslint/no-explicit-any": "error",
      // A leading underscore marks a parameter a platform file must accept but does not use.
      "@typescript-eslint/no-unused-vars": ["error", { argsIgnorePattern: "^_" }],
      "tsdoc/syntax": "error",
      // TSDoc on every exported function, class and constant (§8.3).
      "jsdoc/require-jsdoc": [
        "error",
        {
          publicOnly: true,
          require: { FunctionDeclaration: true, ClassDeclaration: true, ArrowFunctionExpression: true },
          contexts: [
            "ExportNamedDeclaration > VariableDeclaration",
            "TSInterfaceDeclaration",
            "TSTypeAliasDeclaration",
          ],
        },
      ],
      // Size limits from §8.1.
      "max-lines": ["error", { max: 300, skipBlankLines: true, skipComments: true }],
      "max-lines-per-function": ["error", { max: 40, skipBlankLines: true, skipComments: true }],
      "max-depth": ["error", 3],
      "max-params": ["error", 4],
    },
  },
  {
    // React and React Native code: hooks rules, React best practices, and
    // accessibility checks for every interactive element (spec §4.12).
    files: ["apps/client/**/*.{ts,tsx}", "packages/ui/**/*.{ts,tsx}"],
    ...eslintReact.configs["recommended-type-checked"],
  },
  {
    files: ["apps/client/**/*.{ts,tsx}", "packages/ui/**/*.{ts,tsx}"],
    plugins: { "react-hooks": reactHooks, "react-native-a11y": reactNativeA11y },
    languageOptions: { globals: { ...globals.browser } },
    rules: {
      ...reactHooks.configs.recommended.rules,
      ...reactNativeA11y.configs.all.rules,
      // A hint is for extra context; requiring one on every control makes
      // screen readers repeat the label. Hints are added where they help.
      "react-native-a11y/has-accessibility-hint": "off",
    },
  },
  {
    // Every user-facing string comes from the translation files (spec §4.13).
    files: ["apps/client/app/**/*.tsx", "apps/client/src/**/*.tsx"],
    plugins: { i18next },
    rules: {
      "i18next/no-literal-string": [
        "error",
        {
          mode: "jsx-only",
          // Only props a person reads or hears; test IDs and input types are not text.
          "jsx-attributes": {
            include: ["label", "hint", "title", "error", "accessibilityLabel", "aria-label", "placeholder"],
          },
        },
      ],
    },
  },
  {
    // Test files: describe() blocks hold many tests, so their callback is
    // long by nature; the per-test limit still applies to the tests inside.
    files: ["**/*.test.ts", "**/*.test.tsx"],
    rules: {
      "max-lines-per-function": "off",
    },
  },
  {
    // Jest setup files run with Jest's globals.
    files: ["**/jest.setup.js"],
    languageOptions: { globals: { ...globals.jest } },
  },
  {
    // Plain JavaScript config files are not part of any TypeScript project.
    files: ["**/*.mjs", "**/*.cjs", "**/*.js"],
    ...tseslint.configs.disableTypeChecked,
  },
);
