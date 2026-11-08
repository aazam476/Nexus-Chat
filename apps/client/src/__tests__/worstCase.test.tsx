/**
 * Every screen with the permanent worst-case fixtures (code standards
 * §8.5): nothing a user chose is shortened, dropped or garbled. Layout
 * (wrapping, clipping) is checked in the browser by tests/e2e's visual
 * checks, which use the same data.
 */
import { demo, rightToLeft, worstCase, type Dataset } from "@nexus-chat/fixtures";
import { screen } from "@testing-library/react-native";

import Admin from "../../app/admin";
import Home from "../../app/home";
import Login from "../../app/login";
import { isolate } from "../i18n/bidi";
import { fakeServer, type FakeServer } from "../testing/fakeServer";
import { fakeConnection, renderWithApp } from "../testing/renderWithApp";

function serverFor(dataset: Dataset): FakeServer {
  const server = fakeServer();
  server.answer("GET /api/v1/auth/oidc/providers", { status: 200, body: dataset.providers });
  server.answer("GET /api/v1/admin/tls", { status: 200, body: dataset.tls });
  server.answer("GET /api/v1/admin/smtp", { status: 200, body: dataset.smtp });
  return server;
}

function connectionFor(dataset: Dataset, signedIn: boolean) {
  const connection = fakeConnection(serverFor(dataset), signedIn ? { user: dataset.user } : {});
  connection.instance = { info: dataset.instance, setupRequired: false, domainFromEnvironment: false };
  return connection;
}

describe.each([demo, worstCase, rightToLeft])("with $name data", (dataset) => {
  beforeEach(() => {
    jest.useFakeTimers({ now: new Date("2026-11-08T12:00:00Z"), doNotFake: ["nextTick", "setImmediate"] });
  });
  afterEach(() => {
    jest.useRealTimers();
  });

  it("the login screen shows the instance and every provider in full", async () => {
    await renderWithApp(<Login />, connectionFor(dataset, false));

    expect(screen.getByText(`Log in to ${isolate(dataset.instance.name)}`)).toBeOnTheScreen();
    for (const provider of dataset.providers) {
      expect(await screen.findByText(`Continue with ${isolate(provider.displayName)}`)).toBeOnTheScreen();
    }
    expect(screen.queryByTestId("https-warning") !== null).toBe(!dataset.instance.secureConnection);
  });

  it("the home screen shows the whole name and custom status", async () => {
    await renderWithApp(<Home />, connectionFor(dataset, true));

    expect(screen.getByText(`Welcome, ${isolate(dataset.user.displayName)}`)).toBeOnTheScreen();
    const custom = dataset.user.customStatus;
    if (custom === undefined) {
      expect(screen.getByText(/No custom status/)).toBeOnTheScreen();
    } else {
      expect(screen.getByTestId("custom-status")).toHaveTextContent(custom.text);
    }
  });

  it("the Admin panel lists every certificate name and the whole issuer", async () => {
    await renderWithApp(<Admin />, connectionFor(dataset, true));

    for (const name of dataset.tls.names ?? []) {
      expect(await screen.findByText(name)).toBeOnTheScreen();
    }
    expect(screen.getByText(`Issued by: ${dataset.tls.issuer ?? ""}`)).toBeOnTheScreen();
    expect((await screen.findByTestId("smtp-host")).props.value).toBe(dataset.smtp.host);
  });
});

it("says 1 day, not 1 days, when the certificate expires tomorrow", async () => {
  jest.useFakeTimers({ now: new Date("2026-11-08T12:00:00Z"), doNotFake: ["nextTick", "setImmediate"] });
  await renderWithApp(<Admin />, connectionFor(worstCase, true));

  expect(
    await screen.findByText("Warning: the certificate expires in 1 day. Replace it now."),
  ).toBeOnTheScreen();
  jest.useRealTimers();
});
