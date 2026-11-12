import QuickCrypto from "react-native-quick-crypto";

import { installWebCrypto } from "../platform/webCrypto";
import { installWebCrypto as installOnWeb } from "../platform/webCrypto.web";

describe("WebCrypto on the device", () => {
  it("installs the native implementation where the engine has no crypto object", () => {
    const target: { crypto?: unknown } = {};
    installWebCrypto(target);
    expect(target.crypto).toBe(QuickCrypto);
  });

  it("replaces a crypto object that lacks subtle or random UUIDs", () => {
    const partial: { crypto?: unknown } = { crypto: { getRandomValues: () => undefined } };
    installWebCrypto(partial);
    expect(partial.crypto).toBe(QuickCrypto);
  });

  it("keeps a complete WebCrypto the engine already has", () => {
    const own = { subtle: {}, getRandomValues: () => undefined, randomUUID: () => "id" };
    const target: { crypto?: unknown } = { crypto: own };
    installWebCrypto(target);
    expect(target.crypto).toBe(own);
    installOnWeb();
  });
});
