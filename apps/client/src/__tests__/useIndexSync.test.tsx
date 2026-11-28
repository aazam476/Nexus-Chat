import { act, renderHook } from "@testing-library/react-native";

import type { Messenger } from "../messaging/messenger";
import { useIndexSync } from "../search/useIndexSync";

function fakeMessenger() {
  return {
    syncSearchIndex: jest.fn().mockResolvedValue({ fetched: 0, shared: 0 }),
    searchOlder: jest.fn().mockResolvedValue({ indexed: 0, done: true }),
    audienceList: () => [
      { serverId: "s", audienceId: "a", channelIds: ["channel-1"], mayReadHistory: () => true },
    ],
  };
}

async function after(milliseconds: number): Promise<void> {
  await act(async () => {
    jest.advanceTimersByTime(milliseconds);
    await Promise.resolve();
    await Promise.resolve();
  });
}

describe("keeping the search index shared", () => {
  beforeEach(() => {
    jest.useFakeTimers();
  });
  afterEach(() => {
    jest.useRealTimers();
    delete (globalThis as { nexusDesktop?: unknown }).nexusDesktop;
  });

  it("shares soon after messaging starts, then every ten minutes", async () => {
    const messenger = fakeMessenger();
    const { unmount } = await renderHook(() => {
      useIndexSync(messenger as unknown as Messenger, ["dm"]);
    });
    await after(4999);
    expect(messenger.syncSearchIndex).not.toHaveBeenCalled();
    await after(1);
    expect(messenger.syncSearchIndex).toHaveBeenCalledTimes(1);
    await after(10 * 60_000);
    expect(messenger.syncSearchIndex).toHaveBeenCalledTimes(2);
    expect(messenger.searchOlder).not.toHaveBeenCalled();
    await unmount();
    await after(10 * 60_000);
    expect(messenger.syncSearchIndex).toHaveBeenCalledTimes(2);
  });

  it("indexes everything first in the desktop app", async () => {
    (globalThis as { nexusDesktop?: unknown }).nexusDesktop = {};
    const messenger = fakeMessenger();
    await renderHook(() => {
      useIndexSync(messenger as unknown as Messenger, ["dm"]);
    });
    await after(5000);
    await after(0);
    expect(messenger.searchOlder.mock.calls.map((call: unknown[]) => String(call[0]))).toEqual([
      "dm",
      "channel-1",
    ]);
    expect(messenger.syncSearchIndex).toHaveBeenCalled();
  });

  it("does nothing before messaging is ready", async () => {
    await renderHook(() => {
      useIndexSync(undefined, []);
    });
    await after(60_000);
  });
});
