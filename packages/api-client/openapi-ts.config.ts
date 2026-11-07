import { defineConfig } from "@hey-api/openapi-ts";

/**
 * Generator settings. The input is the server's OpenAPI file, which is the
 * single source of truth for the API (spec §2).
 */
export default defineConfig({
  input: "../../apps/server/api/openapi.yaml",
  output: { path: "src/generated" },
  plugins: ["@hey-api/client-fetch", "@hey-api/typescript", "@hey-api/sdk"],
});
