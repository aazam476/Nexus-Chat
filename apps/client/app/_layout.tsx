import { ThemeProvider } from "@nexus-chat/ui";
import { Stack } from "expo-router";
import { StatusBar } from "expo-status-bar";
import { useState, type ReactNode } from "react";
import { I18nextProvider } from "react-i18next";
import { GestureHandlerRootView } from "react-native-gesture-handler";
import { SafeAreaProvider } from "react-native-safe-area-context";

import { ConnectionProvider } from "../src/connection/ConnectionContext";
import { createI18n } from "../src/i18n/i18n";
import { GatewayProvider } from "../src/realtime/GatewayContext";
import { createSecureStorage } from "../src/storage/platformStorage";

/**
 * The root of every screen: gestures, safe areas, theme, translations and
 * the connection to the instance. Screens change with the platform's own
 * transitions, which also honour reduced motion.
 */
export default function RootLayout(): ReactNode {
  const [i18n] = useState(createI18n);
  const [storage] = useState(createSecureStorage);
  return (
    <GestureHandlerRootView style={{ flex: 1 }}>
      <SafeAreaProvider>
        <ThemeProvider preference="system">
          <I18nextProvider i18n={i18n}>
            <ConnectionProvider storage={storage}>
              <GatewayProvider>
                <StatusBar style="auto" />
                <Stack screenOptions={{ headerShown: false }} />
              </GatewayProvider>
            </ConnectionProvider>
          </I18nextProvider>
        </ThemeProvider>
      </SafeAreaProvider>
    </GestureHandlerRootView>
  );
}
