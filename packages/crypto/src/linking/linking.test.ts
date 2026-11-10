import sodium from "libsodium-wrappers-sumo";

import {
  answerLinking,
  confirmLinking,
  finishLinking,
  LinkingError,
  openFromLinkedDevice,
  sealForLinkedDevice,
  startLinking,
} from "./linking.js";

const text = (value: string): Uint8Array => new TextEncoder().encode(value);

describe("linking a new device", () => {
  it("by QR code: both screens show the same code and the keys arrive", async () => {
    const { offer, link: newDevice } = await startLinking();
    const { answer, link: existingDevice } = await answerLinking({ qrCode: offer.qrCode });
    const { reveal, session: onNew } = await finishLinking(newDevice, answer);
    const onExisting = await confirmLinking(existingDevice, reveal);

    expect(offer.qrCode).toMatch(/^nxlink1:[A-Za-z0-9_-]+$/);
    expect(onNew.comparisonCode).toMatch(/^(\d{4} ){5}\d{4}$/);
    expect(onExisting.comparisonCode).toBe(onNew.comparisonCode);
    const sent = await sealForLinkedDevice(onExisting, text("self-signing key and backup key"));
    expect(new TextDecoder().decode(await openFromLinkedDevice(onNew, sent))).toBe(
      "self-signing key and backup key",
    );
    // And the other way.
    const back = await sealForLinkedDevice(onNew, text("thanks"));
    expect(new TextDecoder().decode(await openFromLinkedDevice(onExisting, back))).toBe("thanks");
  });

  it("through the server: the commitment holds the new device to its key", async () => {
    const { offer, link: newDevice } = await startLinking();
    const { answer, link: existingDevice } = await answerLinking({ commitment: offer.commitment });
    const { reveal, session: onNew } = await finishLinking(newDevice, answer);
    const onExisting = await confirmLinking(existingDevice, reveal);

    expect(onExisting.comparisonCode).toBe(onNew.comparisonCode);
    // A relay that swaps in its own key after seeing the answer is caught.
    const relay = await startLinking();
    await expect(
      confirmLinking(existingDevice, { ...reveal, publicKey: relay.link.publicKey }),
    ).rejects.toThrow("the new device's key is not the one it offered");
    await expect(confirmLinking(existingDevice, { ...reveal, nonce: new Uint8Array(16) })).rejects.toThrow(
      LinkingError,
    );
    // A side that never saw an offer accepts nothing.
    await expect(
      confirmLinking({ publicKey: existingDevice.publicKey, secretKey: existingDevice.secretKey }, reveal),
    ).rejects.toThrow(LinkingError);
  });

  it("refuses a key other than the scanned one", async () => {
    const { offer } = await startLinking();
    const other = await startLinking();
    const { answer, link: existingDevice } = await answerLinking({ qrCode: offer.qrCode });
    const { reveal } = await finishLinking(other.link, answer);

    await expect(confirmLinking(existingDevice, reveal)).rejects.toThrow(LinkingError);
    await expect(
      confirmLinking(existingDevice, { publicKey: reveal.publicKey.slice(), nonce: other.link.nonce }),
    ).rejects.toThrow(LinkingError);
  });

  it("shows different codes when someone sits in the middle", async () => {
    // The relay answers the new device with its own key, and offers its own key to the existing one.
    const { link: newDevice } = await startLinking();
    const relay = await startLinking();
    const relayAnswer = await answerLinking({ qrCode: relay.offer.qrCode });
    const { session: onNew } = await finishLinking(newDevice, relayAnswer.answer);
    const { answer, link: existingDevice } = await answerLinking({ qrCode: relay.offer.qrCode });
    const { reveal } = await finishLinking(relay.link, answer);
    const onExisting = await confirmLinking(existingDevice, reveal);

    expect(onExisting.comparisonCode).not.toBe(onNew.comparisonCode);
  });

  it("refuses codes that are not linking codes, and answers that are not keys", async () => {
    await sodium.ready;
    await expect(answerLinking({ qrCode: "https://example.com" })).rejects.toThrow(
      "that is not a Nexus Chat linking code",
    );
    await expect(answerLinking({ qrCode: "nxlink1:!!!" })).rejects.toThrow(LinkingError);
    await expect(answerLinking({ qrCode: "nxlink1:AAAA" })).rejects.toThrow(LinkingError);
    const { link } = await startLinking();
    await expect(finishLinking(link, new Uint8Array(5))).rejects.toThrow(
      "the other device's answer is not a key",
    );
  });

  it("refuses a transfer that was changed or meant for someone else", async () => {
    const { offer, link: newDevice } = await startLinking();
    const { answer, link: existingDevice } = await answerLinking({ qrCode: offer.qrCode });
    const { reveal, session: onNew } = await finishLinking(newDevice, answer);
    const onExisting = await confirmLinking(existingDevice, reveal);
    const sent = await sealForLinkedDevice(onExisting, text("keys"));
    const changed = sent.slice();
    changed[30] = (changed[30] ?? 0) ^ 1;

    await expect(openFromLinkedDevice(onNew, changed)).rejects.toThrow("the transfer could not be opened");
    // The sender can't open its own message: each direction has its own key.
    await expect(openFromLinkedDevice(onExisting, sent)).rejects.toThrow(LinkingError);
  });
});
