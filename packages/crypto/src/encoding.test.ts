import { lengthPrefixed, readFields, uint64, utf8 } from "./encoding.js";

describe("length-prefixed fields", () => {
  it("round-trip, including empty fields", () => {
    const fields = [utf8("label"), new Uint8Array(0), new Uint8Array([1, 2, 3])];

    expect(readFields(lengthPrefixed(...fields), 3)).toEqual(fields);
  });

  it("refuse missing, cut-off and extra bytes", () => {
    const bytes = lengthPrefixed(utf8("ab"), utf8("c"));

    expect(readFields(bytes, 3)).toBeUndefined();
    expect(readFields(bytes.slice(0, -1), 2)).toBeUndefined();
    expect(readFields(new Uint8Array([...bytes, 0]), 2)).toBeUndefined();
    expect(readFields(new Uint8Array([0, 0]), 1)).toBeUndefined();
  });

  it("write numbers big-endian in 8 bytes", () => {
    expect(uint64(258n)).toEqual(new Uint8Array([0, 0, 0, 0, 0, 0, 1, 2]));
  });
});
