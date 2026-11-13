/**
 * How photos are sent by default (ADR 0014): compressed on mobile data and
 * original on Wi-Fi and desktop, unless the user always wants one or the
 * other. Each upload can still be switched. Kept on this device.
 */
import { useCallback, useEffect, useState } from "react";

import { useConnection } from "../connection/ConnectionContext";
import { onMobileData } from "../platform/network";
import type { SecureStorage } from "../storage/storage";

/** The choices, in the order they are offered. */
export const mediaQualities = ["auto", "compress", "original"] as const;
/** One of the choices. */
export type MediaQuality = (typeof mediaQualities)[number];

const key = "nexus.mediaQuality";

/** The stored choice, or "auto". */
export async function readMediaQuality(storage: SecureStorage): Promise<MediaQuality> {
  const stored = await storage.get(key);
  return mediaQualities.find((quality) => quality === stored) ?? "auto";
}

/** Whether a new photo should go out compressed. */
export async function compressByDefault(quality: MediaQuality): Promise<boolean> {
  return quality === "auto" ? onMobileData() : quality === "compress";
}

/** The choice, and how to change it. */
export function useMediaQuality(): { quality: MediaQuality; choose: (quality: MediaQuality) => void } {
  const { storage } = useConnection();
  const [quality, setQuality] = useState<MediaQuality>("auto");
  useEffect(() => {
    let current = true;
    void readMediaQuality(storage).then((stored) => {
      if (current) {
        setQuality(stored);
      }
    });
    return () => {
      current = false;
    };
  }, [storage]);
  const choose = useCallback(
    (next: MediaQuality) => {
      setQuality(next);
      void storage.set(key, next);
    },
    [storage],
  );
  return { quality, choose };
}
