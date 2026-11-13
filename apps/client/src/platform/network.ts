/** Whether the phone is on mobile data now, which makes photos compress by default (ADR 0014). */
import { getNetworkStateAsync, NetworkStateType } from "expo-network";

/** True on mobile data; false on Wi-Fi, other networks, or when unknown. */
export async function onMobileData(): Promise<boolean> {
  try {
    return (await getNetworkStateAsync()).type === NetworkStateType.CELLULAR;
  } catch {
    return false;
  }
}
