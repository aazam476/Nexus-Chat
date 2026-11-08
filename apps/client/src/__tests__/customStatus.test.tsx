import { fireEvent, screen, waitFor } from "@testing-library/react-native";

import Home from "../../app/home";
import { isolate } from "../i18n/bidi";
import { fakeServer } from "../testing/fakeServer";
import { ada, fakeConnection, renderWithApp } from "../testing/renderWithApp";

const away = { text: "At the dentist 🦷", expiresAt: "2026-11-08T15:00:00Z" };

describe("the custom status", () => {
  it("is set with an expiry and keeps the chosen status", async () => {
    const server = fakeServer();
    server.answer("PUT /api/v1/users/me/status", {
      status: 200,
      body: { ...ada, status: "dnd", customStatus: away },
    });
    const connection = await renderWithApp(
      <Home />,
      fakeConnection(server, { user: { ...ada, status: "dnd" } }),
    );
    expect(screen.getByText(/No custom status/)).toBeOnTheScreen();

    await fireEvent.changeText(screen.getByTestId("custom-text"), "At the dentist 🦷");
    await fireEvent.press(screen.getByTestId("custom-expiry-240"));
    await fireEvent.press(screen.getByTestId("custom-save"));

    await waitFor(() => {
      expect(connection.setUser).toHaveBeenCalled();
    });
    const body = server.requests.at(-1)?.body as {
      status: string;
      customStatus: { text: string; expiresAt: string };
    };
    expect(body.status).toBe("dnd");
    expect(body.customStatus.text).toBe("At the dentist 🦷");
    const minutesAhead = (new Date(body.customStatus.expiresAt).getTime() - Date.now()) / 60_000;
    expect(minutesAhead).toBeGreaterThan(239);
    expect(minutesAhead).toBeLessThanOrEqual(240);
    expect(screen.getByTestId("custom-text").props.value).toBe("");
  });

  it("shows when it clears, is kept when the status changes, and can be cleared", async () => {
    const server = fakeServer();
    server.answer("PUT /api/v1/users/me/status", { status: 200, body: ada });
    await renderWithApp(<Home />, fakeConnection(server, { user: { ...ada, customStatus: away } }));
    expect(screen.getByTestId("custom-status")).toHaveTextContent("At the dentist 🦷");
    expect(screen.getByText(/^Clears /)).toBeOnTheScreen();

    await fireEvent.press(screen.getByTestId("status-idle"));
    await waitFor(() => {
      expect(server.requests.at(-1)?.body).toEqual({ status: "idle", customStatus: away });
    });

    await fireEvent.press(screen.getByTestId("custom-clear"));
    await waitFor(() => {
      expect(server.requests.at(-1)?.body).toEqual({ status: "online" });
    });
  });

  it("shows the server's reason for a rejected text, and other failures generically", async () => {
    const server = fakeServer();
    server.answer("PUT /api/v1/users/me/status", {
      status: 422,
      body: {
        code: "validation_failed",
        message: "x",
        fields: [{ field: "customStatus.text", message: "must be 1–128 characters" }],
      },
    });
    await renderWithApp(<Home />, fakeConnection(server, { user: ada }));
    await fireEvent.press(screen.getByTestId("custom-save"));
    expect(await screen.findByText("must be 1–128 characters")).toBeOnTheScreen();

    server.answer("PUT /api/v1/users/me/status", {
      status: 500,
      body: { code: "internal_error", message: "x" },
    });
    await fireEvent.press(screen.getByTestId("custom-save"));
    expect(await screen.findAllByText("Something went wrong. Try again in a moment.")).not.toHaveLength(0);
  });

  it("isolates names so their direction stays inside them", () => {
    expect(isolate("عبد")).toBe("⁨عبد⁩");
  });
});
