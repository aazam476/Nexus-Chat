/**
 * Renders a screen or component with everything the app's root provides:
 * theme, translations, safe areas and a connection, which tests control.
 */
import type { CurrentUser, InstanceInfo } from "@nexus-chat/api-client";
import { ThemeProvider } from "@nexus-chat/ui";
import { render } from "@testing-library/react-native";
import type { ReactElement } from "react";
import { I18nextProvider } from "react-i18next";
import { SafeAreaProvider } from "react-native-safe-area-context";

import { ConnectionContext } from "../connection/ConnectionContext";
import type { Connection } from "../connection/useConnectionState";
import { createI18n } from "../i18n/i18n";
import { MessagingContext } from "../messaging/MessagingContext";
import type { Messaging, MessagingStatus } from "../messaging/useMessagingState";
import { GatewayProvider } from "../realtime/GatewayContext";
import { createMemoryStorage } from "../storage/storage";
import { fakeServer, type FakeServer } from "./fakeServer";

/** A secure, open instance named Example. */
export const exampleInstance: InstanceInfo = {
  name: "Example",
  registrationMode: "open",
  secureConnection: true,
  signingKey: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
  limits: { groupDmMaxMembers: 25 },
  domain: "chat.example.com",
};

/** Ada, a verified user who is online. */
export const ada: CurrentUser = {
  id: "0195f0e0-0000-7000-8000-000000000001",
  username: "ada",
  displayName: "Ada",
  email: "ada@example.com",
  emailVerified: true,
  isInstanceAdmin: false,
  bio: "",
  status: "online",
};

/** A connection to a fake instance; `overrides` change any part of it. */
export function fakeConnection(server: FakeServer, overrides: Partial<Connection> = {}): Connection {
  return {
    storage: createMemoryStorage(),
    loading: false,
    address: "https://chat.example.com",
    client: server.client,
    instance: { info: exampleInstance, setupRequired: false, domainFromEnvironment: false },
    user: undefined,
    sessionToken: undefined,
    chooseInstance: jest.fn().mockResolvedValue(true),
    forgetInstance: jest.fn().mockResolvedValue(undefined),
    signedIn: jest.fn().mockResolvedValue(undefined),
    signOut: jest.fn().mockResolvedValue(undefined),
    refreshInstance: jest.fn().mockResolvedValue(undefined),
    setUser: jest.fn(),
    ...overrides,
  };
}

/** Messaging in the given state; `overrides` change any part of it. */
export function fakeMessaging(
  overrides: Partial<Messaging> & MessagingStatus = { status: "off" },
): Messaging {
  return {
    restore: jest.fn().mockResolvedValue(false),
    confirmPhrase: jest.fn().mockResolvedValue(undefined),
    retry: jest.fn(),
    ...overrides,
  };
}

/** Renders `ui` inside the app's providers with `connection` and `messaging`. */
export async function renderWithApp(
  ui: ReactElement,
  connection: Connection = fakeConnection(fakeServer()),
  messaging: Messaging = fakeMessaging(),
): Promise<Connection> {
  await render(
    <SafeAreaProvider
      initialMetrics={{
        frame: { x: 0, y: 0, width: 400, height: 800 },
        insets: { top: 0, left: 0, right: 0, bottom: 0 },
      }}
    >
      <ThemeProvider preference="light">
        <I18nextProvider i18n={createI18n()}>
          <ConnectionContext value={connection}>
            <GatewayProvider>
              <MessagingContext value={messaging}>{ui}</MessagingContext>
            </GatewayProvider>
          </ConnectionContext>
        </I18nextProvider>
      </ThemeProvider>
    </SafeAreaProvider>,
  );
  return connection;
}
