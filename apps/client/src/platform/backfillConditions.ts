/**
 * Whether a phone may backfill old messages for search now (spec §4.6):
 * by default only on Wi-Fi and while charging, so indexing years of
 * history costs neither data nor battery.
 */
import { BatteryState, getBatteryStateAsync } from "expo-battery";
import { getNetworkStateAsync, NetworkStateType } from "expo-network";

/** True on Wi-Fi (or Ethernet) while charging or full; false otherwise, or when unknown. */
export async function goodTimeToBackfill(): Promise<boolean> {
  try {
    const [network, battery] = await Promise.all([getNetworkStateAsync(), getBatteryStateAsync()]);
    const unmetered = network.type === NetworkStateType.WIFI || network.type === NetworkStateType.ETHERNET;
    const powered = battery === BatteryState.CHARGING || battery === BatteryState.FULL;
    return unmetered && powered;
  } catch {
    return false;
  }
}
