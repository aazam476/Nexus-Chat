import { fireEvent, screen, waitFor } from "@testing-library/react-native";
import { router } from "expo-router";

import Connect from "../../app/connect";
import Start from "../../app/index";
import Register from "../../app/register";
import Setup from "../../app/setup";
import { fakeServer } from "../testing/fakeServer";
import { ada, exampleInstance, fakeConnection, renderWithApp } from "../testing/renderWithApp";

const session = {
  id: "0195f0e0-0000-7000-8000-0000000000aa",
  deviceName: "iOS app",
  createdAt: "2026-11-08T12:00:00Z",
  lastSeenAt: "2026-11-08T12:00:00Z",
  expiresAt: "2027-02-06T12:00:00Z",
  current: true,
};

beforeEach(() => {
  jest.clearAllMocks();
});

describe("sign-up", () => {
  async function fill(): Promise<void> {
    await fireEvent.changeText(screen.getByTestId("register-username"), " grace ");
    await fireEvent.changeText(screen.getByTestId("register-displayName"), "Grace");
    await fireEvent.changeText(screen.getByTestId("register-email"), "grace@example.com");
    await fireEvent.changeText(screen.getByTestId("register-password"), "a long password");
  }

  it("creates the account and logs it in", async () => {
    const server = fakeServer();
    server.answer("POST /api/v1/auth/register", {
      status: 201,
      body: { token: "nxs_r", session, user: ada },
    });
    const connection = await renderWithApp(<Register />, fakeConnection(server));
    await fill();

    await fireEvent.press(screen.getByTestId("register-submit"));

    await waitFor(() => {
      expect(connection.signedIn).toHaveBeenCalledWith("nxs_r", ada);
    });
    expect(server.requests.at(-1)?.body).toEqual({
      username: "grace",
      displayName: "Grace",
      email: "grace@example.com",
      password: "a long password",
      deviceName: "iOS app",
    });
  });

  it("asks for an invite on invite-only instances and shows field errors", async () => {
    const server = fakeServer();
    server.answer("POST /api/v1/auth/register", {
      status: 422,
      body: {
        code: "validation_failed",
        message: "x",
        fields: [{ field: "username", message: "is already taken" }],
      },
    });
    const connection = fakeConnection(server);
    connection.instance = {
      info: { ...exampleInstance, registrationMode: "invite_only" },
      setupRequired: false,
      domainFromEnvironment: false,
    };
    await renderWithApp(<Register />, connection);
    await fill();
    await fireEvent.changeText(screen.getByTestId("register-inviteCode"), " INVITE ");

    await fireEvent.press(screen.getByTestId("register-submit"));

    expect(await screen.findByText("is already taken")).toBeOnTheScreen();
    expect((server.requests.at(-1)?.body as { inviteCode: string }).inviteCode).toBe("INVITE");
  });

  it("explains a refused registration and links back to login", async () => {
    const server = fakeServer();
    server.answer("POST /api/v1/auth/register", {
      status: 403,
      body: { code: "invite_required", message: "x" },
    });
    await renderWithApp(<Register />, fakeConnection(server));
    await fill();
    await fireEvent.press(screen.getByTestId("register-submit"));
    expect(await screen.findByText("This instance is invite-only. Enter an invite code.")).toBeOnTheScreen();

    await fireEvent.press(screen.getByText("I already have an account"));
    expect(router.replace).toHaveBeenCalledWith("/login");
  });
});

describe("the setup wizard", () => {
  async function fillSetup(): Promise<void> {
    await fireEvent.changeText(screen.getByTestId("setup-token"), " TOKEN ");
    await fireEvent.changeText(screen.getByTestId("setup-username"), "admin");
    await fireEvent.changeText(screen.getByTestId("setup-displayName"), "Admin");
    await fireEvent.changeText(screen.getByTestId("setup-email"), "admin@example.com");
    await fireEvent.changeText(screen.getByTestId("setup-password"), "a long password");
    await fireEvent.changeText(screen.getByTestId("setup-instanceName"), "Example");
  }

  it("sets the instance up and sends the admin to log in", async () => {
    const server = fakeServer();
    server.answer("POST /api/v1/setup", { status: 201, body: { adminUserId: ada.id } });
    const connection = await renderWithApp(<Setup />, fakeConnection(server));
    await fillSetup();
    await fireEvent.changeText(screen.getByTestId("setup-domain"), " chat.example.com ");
    await fireEvent.press(screen.getByTestId("mode-open"));

    await fireEvent.press(screen.getByTestId("setup-submit"));
    await fireEvent.press(await screen.findByTestId("setup-go-login"));

    expect(router.replace).toHaveBeenCalledWith("/login");
    expect(connection.refreshInstance).toHaveBeenCalled();
    expect(server.requests.at(-1)?.body).toEqual({
      setupToken: "TOKEN",
      admin: {
        username: "admin",
        displayName: "Admin",
        email: "admin@example.com",
        password: "a long password",
      },
      instance: { name: "Example", registrationMode: "open", domain: "chat.example.com" },
    });
  });

  it("warns over plain HTTP, shows a fixed domain and explains errors", async () => {
    const server = fakeServer();
    server.answer("POST /api/v1/setup", { status: 401, body: { code: "invalid_setup_token", message: "x" } });
    const connection = fakeConnection(server);
    connection.instance = {
      info: { ...exampleInstance, secureConnection: false },
      setupRequired: true,
      domainFromEnvironment: true,
    };
    await renderWithApp(<Setup />, connection);

    expect(screen.getByTestId("setup-http-warning")).toBeOnTheScreen();
    expect(screen.getByTestId("https-warning")).toBeOnTheScreen();
    expect(screen.getByText("Set by DOMAIN in .env: chat.example.com")).toBeOnTheScreen();
    await fillSetup();
    await fireEvent.press(screen.getByTestId("setup-submit"));
    expect(
      await screen.findByText("That setup token is wrong. Copy it again from the server logs."),
    ).toBeOnTheScreen();
    expect((server.requests.at(-1)?.body as { instance: object }).instance).toEqual({
      name: "Example",
      registrationMode: "invite_only",
    });
  });

  it("shows field errors next to the fields", async () => {
    const server = fakeServer();
    server.answer("POST /api/v1/setup", {
      status: 422,
      body: {
        code: "validation_failed",
        message: "x",
        fields: [{ field: "admin.password", message: "is too short" }],
      },
    });
    await renderWithApp(<Setup />, fakeConnection(server, { address: undefined }));
    await fillSetup();

    await fireEvent.press(screen.getByTestId("setup-submit"));

    expect(await screen.findByText("is too short")).toBeOnTheScreen();
    expect(screen.getByText("Set up localhost")).toBeOnTheScreen();
  });
});

describe("the instance picker", () => {
  it("connects to an instance that answers", async () => {
    const connection = await renderWithApp(<Connect />, fakeConnection(fakeServer()));
    await fireEvent.changeText(screen.getByTestId("instance-address"), "chat.example.com");

    await fireEvent.press(screen.getByTestId("connect"));

    await waitFor(() => {
      expect(router.replace).toHaveBeenCalledWith("/");
    });
    expect(connection.chooseInstance).toHaveBeenCalledWith("https://chat.example.com");
  });

  it("explains an invalid or silent address", async () => {
    const connection = fakeConnection(fakeServer(), { chooseInstance: jest.fn().mockResolvedValue(false) });
    await renderWithApp(<Connect />, connection);

    await fireEvent(screen.getByTestId("instance-address"), "submitEditing");
    expect(await screen.findByText("Enter an address such as chat.example.com.")).toBeOnTheScreen();

    await fireEvent.changeText(screen.getByTestId("instance-address"), "nothing.example.com");
    await fireEvent.press(screen.getByTestId("connect"));
    expect(await screen.findByText("No Nexus Chat instance answered at this address.")).toBeOnTheScreen();
    expect(router.replace).not.toHaveBeenCalled();
  });
});

describe("the start screen", () => {
  it("waits while loading, then redirects", async () => {
    await renderWithApp(<Start />, fakeConnection(fakeServer(), { loading: true }));
    expect(screen.getByText("Loading…")).toBeOnTheScreen();

    await renderWithApp(<Start />, fakeConnection(fakeServer(), { user: ada }));
    expect(screen.getByText("redirect:/home")).toBeOnTheScreen();
  });
});
