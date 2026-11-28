import { BatteryState, getBatteryStateAsync } from "expo-battery";
import { getNetworkStateAsync, NetworkStateType } from "expo-network";

import { goodTimeToBackfill } from "../platform/backfillConditions";
import { goodTimeToBackfill as inBrowsers } from "../platform/backfillConditions.web";

jest.mock("expo-battery", () => ({
  getBatteryStateAsync: jest.fn(),
  BatteryState: { UNKNOWN: 0, UNPLUGGED: 1, CHARGING: 2, FULL: 3 },
}));
jest.mock("expo-network", () => ({
  getNetworkStateAsync: jest.fn(),
  NetworkStateType: { NONE: "NONE", CELLULAR: "CELLULAR", WIFI: "WIFI", ETHERNET: "ETHERNET" },
}));

function on(network: NetworkStateType, battery: BatteryState): void {
  jest.mocked(getNetworkStateAsync).mockResolvedValue({ type: network });
  jest.mocked(getBatteryStateAsync).mockResolvedValue(battery);
}

describe("backfilling older messages on a phone", () => {
  it("goes ahead only on Wi-Fi or Ethernet while charging or full", async () => {
    on(NetworkStateType.WIFI, BatteryState.CHARGING);
    expect(await goodTimeToBackfill()).toBe(true);
    on(NetworkStateType.ETHERNET, BatteryState.FULL);
    expect(await goodTimeToBackfill()).toBe(true);
    on(NetworkStateType.CELLULAR, BatteryState.CHARGING);
    expect(await goodTimeToBackfill()).toBe(false);
    on(NetworkStateType.WIFI, BatteryState.UNPLUGGED);
    expect(await goodTimeToBackfill()).toBe(false);
  });

  it("waits when it cannot tell", async () => {
    jest.mocked(getNetworkStateAsync).mockRejectedValue(new Error("no permission"));
    expect(await goodTimeToBackfill()).toBe(false);
  });

  it("never waits in browsers and the desktop app", async () => {
    expect(await inBrowsers()).toBe(true);
  });
});
