/**
 * The app's configuration: app.json, plus one switch for end-to-end test
 * builds. Android refuses plain-HTTP connections, as it should (spec
 * §3.3); the Maestro flows in CI talk to a throwaway instance over HTTP,
 * so only a build made with NEXUS_E2E_CLEARTEXT=1 allows it. Store builds
 * never set it.
 */
import type { ConfigContext, ExpoConfig } from "expo/config";
import { withAndroidManifest, type ConfigPlugin } from "expo/config-plugins";

const allowCleartextForTests: ConfigPlugin = (config) =>
  withAndroidManifest(config, (modified) => {
    for (const application of modified.modResults.manifest.application ?? []) {
      application.$["android:usesCleartextTraffic"] = "true";
    }
    return modified;
  });

/** The configuration Expo uses for every build. */
export default function appConfig({ config }: ConfigContext): ExpoConfig {
  const base = config as ExpoConfig;
  return process.env.NEXUS_E2E_CLEARTEXT === "1" ? allowCleartextForTests(base) : base;
}
