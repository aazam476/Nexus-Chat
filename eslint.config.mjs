// Shared ESLint configuration (code standards §8.3). Every workspace's
// `pnpm lint` uses this file; the build fails on any error.
import eslint from "@eslint/js";
import jsdoc from "eslint-plugin-jsdoc";
import tsdoc from "eslint-plugin-tsdoc";
import globals from "globals";
import tseslint from "typescript-eslint";

export default tseslint.config(
  {
    ignores: ["**/dist/**", "**/coverage/**", "**/generated/**", "**/node_modules/**"],
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
    // Test files: describe() blocks hold many tests, so their callback is
    // long by nature; the per-test limit still applies to the tests inside.
    files: ["**/*.test.ts", "**/*.test.tsx"],
    rules: {
      "max-lines-per-function": "off",
    },
  },
  {
    // Plain JavaScript config files are not part of any TypeScript project.
    files: ["**/*.mjs", "**/*.cjs", "**/*.js"],
    ...tseslint.configs.disableTypeChecked,
  },
);
