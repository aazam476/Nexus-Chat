/**
 * A name for this device in the session list ("Firefox on Linux",
 * "iOS app"), so the user can tell their sessions apart.
 */
import { Platform } from "react-native";

const browsers: [RegExp, string][] = [
  [/Edg\//, "Edge"],
  [/Firefox\//, "Firefox"],
  [/Chrome\//, "Chrome"],
  [/Safari\//, "Safari"],
];
const systems: [RegExp, string][] = [
  [/Windows/, "Windows"],
  [/Mac OS X|Macintosh/, "macOS"],
  [/Android/, "Android"],
  [/iPhone|iPad/, "iOS"],
  [/Linux/, "Linux"],
];

/** The device name for this platform; `userAgent` is the browser's, on web. */
export function deviceName(
  userAgent: string | undefined = typeof navigator === "undefined" ? undefined : navigator.userAgent,
): string {
  if (Platform.OS === "ios") {
    return "iOS app";
  }
  if (Platform.OS === "android") {
    return "Android app";
  }
  const agent = userAgent ?? "";
  const browser = browsers.find(([pattern]) => pattern.test(agent))?.[1] ?? "Browser";
  const system = systems.find(([pattern]) => pattern.test(agent))?.[1];
  const desktop = agent.includes("Electron/") ? "Nexus Chat desktop" : browser;
  return system === undefined ? desktop : `${desktop} on ${system}`;
}
