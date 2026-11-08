import { Platform } from "react-native";

import { deviceName } from "../auth/deviceName";
import { codeOf, fieldMessages, messageFor } from "../auth/errors";
import { parseReturn } from "../auth/providerReturn";
import { hostOf, normalizeAddress } from "../instance/address";
import { defaultAddress } from "../instance/defaultAddress";
import { startPath } from "../navigation/startPath";
import { exampleInstance, ada } from "../testing/renderWithApp";

describe("normalizeAddress", () => {
  it.each([
    ["chat.example.com", "https://chat.example.com"],
    ["  chat.example.com/some/path ", "https://chat.example.com"],
    ["http://192.168.1.10:8080", "http://192.168.1.10:8080"],
    ["localhost:8080", "http://localhost:8080"],
    ["127.0.0.1", "http://127.0.0.1"],
    ["[::1]:8080", "http://[::1]:8080"],
    ["HTTPS://Chat.Example.com", "https://chat.example.com"],
  ])("turns %s into %s", (input, expected) => {
    expect(normalizeAddress(input)).toBe(expected);
  });

  it.each(["", "   ", "chat example.com", "ftp://chat.example.com", "https://", "http://[bad"])(
    "rejects %p",
    (input) => {
      expect(normalizeAddress(input)).toBeUndefined();
    },
  );

  it("shows the host of an address", () => {
    expect(hostOf("https://chat.example.com:8443")).toBe("chat.example.com:8443");
  });
});

describe("deviceName", () => {
  const originalOS = Platform.OS;
  afterEach(() => {
    Platform.OS = originalOS;
  });

  it.each([
    ["Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 Chrome/140.0 Safari/537.36", "Chrome on Linux"],
    ["Mozilla/5.0 (Windows NT 10.0) Gecko/20100101 Firefox/146.0", "Firefox on Windows"],
    [
      "Mozilla/5.0 (Macintosh; Intel Mac OS X 15_0) AppleWebKit/605 Version/19.0 Safari/605",
      "Safari on macOS",
    ],
    ["Mozilla/5.0 (Windows NT 10.0) Chrome/140.0 Safari/537.36 Edg/140.0", "Edge on Windows"],
    ["Mozilla/5.0 (Linux) Chrome/140 Electron/45.2.3 Safari/537.36", "Nexus Chat desktop on Linux"],
    ["SomethingElse", "Browser"],
  ])("names %s", (agent, expected) => {
    Platform.OS = "web";
    expect(deviceName(agent)).toBe(expected);
  });

  it("names the native apps", () => {
    Platform.OS = "ios";
    expect(deviceName()).toBe("iOS app");
    Platform.OS = "android";
    expect(deviceName()).toBe("Android app");
  });

  it("uses the browser's own user agent by default", () => {
    Platform.OS = "web";
    expect(typeof deviceName()).toBe("string");
    expect(deviceName(undefined)).toBe(deviceName(navigator.userAgent));
  });
});

describe("errors", () => {
  it("maps known codes and falls back for the rest", () => {
    expect(
      messageFor(
        { code: "invalid_credentials", message: "x" },
        { invalid_credentials: "login.invalidCredentials" },
      ),
    ).toEqual({ key: "login.invalidCredentials" });
    expect(messageFor({ code: "other", message: "x" }, {})).toEqual({ key: "common.unexpectedError" });
    expect(messageFor(new Error("network"), {})).toEqual({ key: "common.unexpectedError" });
    expect(codeOf(null)).toBeUndefined();
    expect(codeOf({ code: 4 })).toBeUndefined();
  });

  it("collects field messages", () => {
    expect(
      fieldMessages({
        code: "validation_failed",
        message: "",
        fields: [{ field: "email", message: "is invalid" }],
      }),
    ).toEqual({ email: "is invalid" });
    expect(fieldMessages(undefined)).toEqual({});
  });
});

describe("startPath", () => {
  const instance = { info: exampleInstance, setupRequired: false, domainFromEnvironment: false };

  it("picks the screen for the connection", () => {
    expect(startPath({ address: undefined, instance: undefined, user: undefined })).toBe("/connect");
    expect(startPath({ address: "https://a", instance: undefined, user: undefined })).toBe("/connect");
    expect(
      startPath({ address: "https://a", instance: { ...instance, setupRequired: true }, user: undefined }),
    ).toBe("/setup");
    expect(startPath({ address: "https://a", instance, user: undefined })).toBe("/login");
    expect(startPath({ address: "https://a", instance, user: ada })).toBe("/home");
  });
});

describe("provider return", () => {
  it("reads the code or the error from the fragment", () => {
    expect(parseReturn("#flow=f&code=c")).toEqual({ code: "c" });
    expect(parseReturn("error=provider_error")).toEqual({ error: "provider_error" });
    expect(parseReturn("")).toEqual({});
  });
});

describe("defaultAddress", () => {
  it("has none on native", () => {
    expect(defaultAddress()).toBeUndefined();
  });
});
