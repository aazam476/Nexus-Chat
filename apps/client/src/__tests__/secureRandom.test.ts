import { installSecureRandom } from "../platform/secureRandom";

jest.mock("expo-crypto", () => ({
  getRandomValues: (array: Uint8Array) => array.fill(7),
}));

describe("the secure random source", () => {
  it("adds getRandomValues where the engine has no crypto object", () => {
    const target: { crypto?: unknown } = {};
    installSecureRandom(target);
    const crypto = target.crypto as { getRandomValues: (array: Uint8Array) => Uint8Array };
    expect(crypto.getRandomValues(new Uint8Array(2))).toEqual(new Uint8Array([7, 7]));
  });

  it("adds it to a crypto object that lacks it, keeping the rest", () => {
    const subtle = {};
    const target: { crypto?: unknown } = { crypto: { subtle } };
    installSecureRandom(target);
    const crypto = target.crypto as { subtle: unknown; getRandomValues: unknown };
    expect(crypto.subtle).toBe(subtle);
    expect(typeof crypto.getRandomValues).toBe("function");
  });

  it("keeps a getRandomValues the engine already has", () => {
    const own = (array: Uint8Array): Uint8Array => array;
    const target: { crypto?: unknown } = { crypto: { getRandomValues: own } };
    installSecureRandom(target);
    expect((target.crypto as { getRandomValues: unknown }).getRandomValues).toBe(own);
  });
});
