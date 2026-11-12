import {
  contentSecurityPolicy,
  isAppAddress,
  isExternalLink,
  isWebPage,
  providerReturnAddress,
  providerReturnFragment,
} from "./navigation";

describe("isAppAddress", () => {
  it.each([
    ["app://nexus-chat/login", true],
    ["app://nexus-chat/", true],
    ["app://other/", false],
    ["https://nexus-chat/", false],
    ["https://evil.example.com/", false],
    ["file:///etc/passwd", false],
    ["not a url", false],
  ])("%s → %s", (url, expected) => {
    expect(isAppAddress(url)).toBe(expected);
  });
});

describe("isExternalLink", () => {
  it.each([
    ["https://example.com/page", true],
    ["http://example.com/", true],
    ["mailto:ada@example.com", true],
    ["file:///etc/passwd", false],
    ["javascript:alert(1)", false],
    ["smb://share/x", false],
    ["", false],
  ])("%s → %s", (url, expected) => {
    expect(isExternalLink(url)).toBe(expected);
  });
});

describe("isWebPage", () => {
  it.each([
    ["https://idp.example.com/authorize", true],
    ["http://idp.local/", true],
    ["mailto:ada@example.com", false],
    ["file:///etc/passwd", false],
    ["nonsense", false],
  ])("%s → %s", (url, expected) => {
    expect(isWebPage(url)).toBe(expected);
  });
});

describe("providerReturnFragment", () => {
  it("returns the fragment of the return address", () => {
    expect(providerReturnFragment(`${providerReturnAddress}#flow=f&code=c`)).toBe("flow=f&code=c");
  });

  it("returns an empty fragment when there is none", () => {
    expect(providerReturnFragment(providerReturnAddress)).toBe("");
  });

  it.each([
    "https://idp.example.com/authorize?redirect=nexuschat://oidc-return",
    "nexuschat://oidc-return/extra#code=c",
    "nexuschat://oidc-return?x=1#code=c",
    "https://chat.example.com/oidc-return#code=c",
  ])("ignores %s", (url) => {
    expect(providerReturnFragment(url)).toBeUndefined();
  });
});

describe("contentSecurityPolicy", () => {
  it("allows scripts only from the bundle", () => {
    expect(contentSecurityPolicy).toContain("script-src 'self' 'wasm-unsafe-eval';");
    expect(contentSecurityPolicy).not.toContain("'unsafe-eval'");
    expect(contentSecurityPolicy).toContain("object-src 'none'");
  });
});
