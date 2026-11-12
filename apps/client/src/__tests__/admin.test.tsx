import { fireEvent, screen, waitFor } from "@testing-library/react-native";
import { router } from "expo-router";

import Admin from "../../app/admin";
import Home from "../../app/home";
import { expiryWarning } from "../admin/expiry";
import { fakeServer, type FakeServer } from "../testing/fakeServer";
import { ada, fakeConnection, renderWithApp } from "../testing/renderWithApp";

const admin = { ...ada, isInstanceAdmin: true };
const day = 24 * 60 * 60 * 1000;
const unconfiguredSmtp = {
  configured: false,
  host: "",
  port: 0,
  security: "starttls",
  username: "",
  from: "",
  passwordSet: false,
};

function adminServer(tls: object = { active: false }, smtp: object = unconfiguredSmtp): FakeServer {
  const server = fakeServer();
  server.answer("GET /api/v1/admin/tls", { status: 200, body: tls });
  server.answer("GET /api/v1/admin/smtp", { status: 200, body: smtp });
  server.answer("GET /api/v1/admin/uploads", { status: 200, body: { attachmentMaxBytes: 25 << 20 } });
  return server;
}

async function openAdmin(server: FakeServer): Promise<void> {
  await renderWithApp(<Admin />, fakeConnection(server, { user: admin }));
  await screen.findByTestId("tls-state");
}

describe("expiry warnings", () => {
  const now = new Date("2026-11-08T12:00:00Z");
  const inDays = (days: number): string => new Date(now.getTime() + days * day).toISOString();

  it.each([
    [undefined, undefined],
    [inDays(31), undefined],
    [inDays(30), { daysLeft: 30, urgent: false }],
    [inDays(7), { daysLeft: 7, urgent: true }],
    [inDays(0.5), { daysLeft: 0, urgent: true }],
    [inDays(-2), { daysLeft: -2, urgent: true }],
  ])("for a certificate expiring at %s", (expiresAt, expected) => {
    expect(expiryWarning(expiresAt, now)).toEqual(expected);
  });
});

describe("the Admin panel", () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it("is only for instance admins, who find it from home", async () => {
    await renderWithApp(<Admin />, fakeConnection(adminServer(), { user: ada }));
    expect(router.replace).toHaveBeenCalledWith("/");
    expect(screen.queryByTestId("tls-state")).toBeNull();

    await renderWithApp(<Home />, fakeConnection(fakeServer(), { user: ada }));
    expect(screen.queryByTestId("go-admin")).toBeNull();
    await renderWithApp(<Home />, fakeConnection(fakeServer(), { user: admin }));
    await fireEvent.press(screen.getByTestId("go-admin"));
    expect(router.push).toHaveBeenCalledWith("/admin");
  });

  it("warns about plain HTTP at the top of the page", async () => {
    const server = adminServer();
    const connection = fakeConnection(server, { user: admin });
    connection.instance = {
      info: {
        name: "Example",
        registrationMode: "open",
        secureConnection: false,
        signingKey: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
        limits: { groupDmMaxMembers: 25, attachmentMaxBytes: 26214400 },
        linkPreviewProxy: false,
      },
      setupRequired: false,
      domainFromEnvironment: false,
    };
    await renderWithApp(<Admin />, connection);

    expect(await screen.findByTestId("https-warning")).toBeOnTheScreen();
    await fireEvent.press(screen.getByTestId("admin-back"));
    expect(router.replace).toHaveBeenCalledWith("/home");
  });

  it("installs a certificate and clears the key from the page", async () => {
    const server = adminServer();
    server.answer("PUT /api/v1/admin/tls", {
      status: 200,
      body: {
        active: true,
        names: ["chat.example.com"],
        issuer: "Example CA",
        expiresAt: "2027-02-01T00:00:00Z",
      },
    });
    await openAdmin(server);
    expect(screen.getByText(/No certificate is installed/)).toBeOnTheScreen();

    await fireEvent.changeText(screen.getByTestId("tls-certificate"), "CERT");
    await fireEvent.changeText(screen.getByTestId("tls-private-key"), "KEY");
    await fireEvent.press(screen.getByTestId("tls-install"));

    expect(await screen.findByText("The certificate is installed and HTTPS is on.")).toBeOnTheScreen();
    expect(server.requests.at(-1)?.body).toEqual({ certificate: "CERT", privateKey: "KEY" });
    expect(screen.getByText("Covers these names:")).toBeOnTheScreen();
    expect(screen.getByText("chat.example.com")).toBeOnTheScreen();
    expect(screen.getByText("Issued by: Example CA")).toBeOnTheScreen();
    expect(screen.getByText("Expires: February 1, 2027")).toBeOnTheScreen();
    expect(screen.getByTestId("tls-private-key").props.value).toBe("");
  });

  it("shows why a certificate was rejected and keeps what was typed", async () => {
    const server = adminServer();
    server.answer("PUT /api/v1/admin/tls", {
      status: 422,
      body: { code: "certificate_rejected", message: "the chain is incomplete" },
    });
    await openAdmin(server);
    await fireEvent.changeText(screen.getByTestId("tls-private-key"), "KEY");
    await fireEvent.press(screen.getByTestId("tls-install"));

    expect(
      await screen.findByText("The certificate was not installed: the chain is incomplete"),
    ).toBeOnTheScreen();
    expect(screen.getByTestId("tls-private-key").props.value).toBe("KEY");
  });

  it("warns before expiry and removes a certificate only after confirming", async () => {
    const soon = new Date(Date.now() + 5 * day + 60_000).toISOString();
    const server = adminServer({ active: true, names: ["chat.example.com"], issuer: "CA", expiresAt: soon });
    server.answer("DELETE /api/v1/admin/tls", { status: 204 });
    await openAdmin(server);
    expect(screen.getByText("Warning: the certificate expires in 5 days. Replace it now.")).toBeOnTheScreen();

    await fireEvent.press(screen.getByTestId("tls-remove"));
    await fireEvent.press(screen.getByText("Cancel"));
    expect(server.requests.some((request) => request.method === "DELETE")).toBe(false);

    await fireEvent.press(screen.getByTestId("tls-remove"));
    expect(screen.getByText(/Removing the certificate turns HTTPS off/)).toBeOnTheScreen();
    await fireEvent.press(screen.getByTestId("tls-remove-confirm"));

    expect(await screen.findByText("The certificate is removed and HTTPS is off.")).toBeOnTheScreen();
    expect(screen.queryByTestId("tls-remove")).toBeNull();
  });

  it("reports load and change failures generically", async () => {
    const server = fakeServer();
    server.answer("GET /api/v1/admin/tls", { status: 500, body: { code: "internal_error", message: "x" } });
    server.answer("GET /api/v1/admin/smtp", { status: 500, body: { code: "internal_error", message: "x" } });
    await renderWithApp(<Admin />, fakeConnection(server, { user: admin }));

    expect(await screen.findAllByText("Something went wrong. Try again in a moment.")).toHaveLength(2);
  });
});

describe("the email settings", () => {
  const saved = {
    configured: true,
    host: "smtp.example.com",
    port: 465,
    security: "tls",
    username: "chat",
    from: "chat@example.com",
    passwordSet: true,
  };

  it("start from the saved settings and keep the password unless one is typed", async () => {
    const server = adminServer({ active: false }, saved);
    server.answer("PUT /api/v1/admin/smtp", { status: 200, body: { ...saved, host: "mail.example.com" } });
    await openAdmin(server);
    expect(screen.getByTestId("smtp-host").props.value).toBe("smtp.example.com");
    expect(screen.getByTestId("smtp-port").props.value).toBe("465");
    expect(screen.getByText("✓ TLS (usually port 465)")).toBeOnTheScreen();
    expect(screen.getByText("A password is saved. Leave this empty to keep it.")).toBeOnTheScreen();

    await fireEvent.changeText(screen.getByTestId("smtp-host"), " mail.example.com ");
    await fireEvent.press(screen.getByTestId("smtp-security-starttls"));
    await fireEvent.press(screen.getByTestId("smtp-save"));

    expect(await screen.findByText("Email settings saved.")).toBeOnTheScreen();
    expect(server.requests.at(-1)?.body).toEqual({
      host: "mail.example.com",
      port: 465,
      security: "starttls",
      username: "chat",
      from: "chat@example.com",
    });

    await fireEvent.changeText(screen.getByTestId("smtp-password"), "secret");
    await fireEvent.press(screen.getByTestId("smtp-save"));
    await waitFor(() => {
      expect(server.requests.at(-1)?.body).toMatchObject({ password: "secret" });
    });
  });

  it("suggest the usual port for a new setup and show field errors", async () => {
    const server = adminServer();
    server.answer("PUT /api/v1/admin/smtp", {
      status: 422,
      body: {
        code: "validation_failed",
        message: "x",
        fields: [{ field: "from", message: "Not an email address." }],
      },
    });
    await openAdmin(server);
    expect(screen.getByTestId("smtp-port").props.value).toBe("587");
    expect(screen.getByText("Leave empty if the mail server needs no sign-in.")).toBeOnTheScreen();

    await fireEvent.changeText(screen.getByTestId("smtp-port"), "not a number");
    await fireEvent.press(screen.getByTestId("smtp-save"));

    expect(await screen.findByText("Not an email address.")).toBeOnTheScreen();
    expect(server.requests.at(-1)?.body).toMatchObject({ port: 0 });
  });

  it.each([
    [{ status: 204 }, "The test email was sent to your address."],
    [
      { status: 422, body: { code: "smtp_not_configured", message: "x" } },
      "Save the email settings before sending a test email.",
    ],
    [
      { status: 502, body: { code: "mail_delivery_failed", message: "x" } },
      "The mail server did not accept the test email. Check the settings and try again.",
    ],
    [
      { status: 429, body: { code: "rate_limited", message: "x" } },
      "Too many test emails. Wait a minute and try again.",
    ],
    [
      { status: 500, body: { code: "internal_error", message: "x" } },
      "Something went wrong. Try again in a moment.",
    ],
  ])("explain a test email outcome", async (answer, text) => {
    const server = adminServer({ active: false }, saved);
    server.answer("POST /api/v1/admin/smtp/test", answer);
    await openAdmin(server);

    await fireEvent.press(screen.getByTestId("smtp-test"));

    expect(await screen.findByText(text)).toBeOnTheScreen();
  });

  it("report a failed save", async () => {
    const server = adminServer({ active: false }, saved);
    server.answer("PUT /api/v1/admin/smtp", { status: 500, body: { code: "internal_error", message: "x" } });
    await openAdmin(server);
    await fireEvent.press(screen.getByTestId("smtp-save"));

    expect(await screen.findByText("Something went wrong. Try again in a moment.")).toBeOnTheScreen();
  });
});

describe("removing a certificate that the server refuses to remove", () => {
  it("shows the failure and keeps the certificate", async () => {
    const server = adminServer({
      active: true,
      names: ["a"],
      issuer: "CA",
      expiresAt: "2099-01-01T00:00:00Z",
    });
    server.answer("DELETE /api/v1/admin/tls", {
      status: 500,
      body: { code: "internal_error", message: "x" },
    });
    await openAdmin(server);
    expect(screen.queryByTestId("tls-expiry-warning")).toBeNull();

    await fireEvent.press(screen.getByTestId("tls-remove"));
    await fireEvent.press(screen.getByTestId("tls-remove-confirm"));

    expect(await screen.findByText("Something went wrong. Try again in a moment.")).toBeOnTheScreen();
    expect(screen.getByTestId("tls-remove")).toBeOnTheScreen();
  });
});

describe("the uploads section", () => {
  it("shows the attachment limit and saves a new one in MB", async () => {
    const server = adminServer();
    server.answer("PUT /api/v1/admin/uploads", { status: 200, body: { attachmentMaxBytes: 100 << 20 } });
    await openAdmin(server);
    expect(await screen.findByText("Files can be up to 25 MB.")).toBeOnTheScreen();

    await fireEvent.changeText(screen.getByTestId("upload-limit-field"), "lots");
    await fireEvent.press(screen.getByTestId("upload-limit-save"));
    expect(await screen.findByText(/whole number of MB/)).toBeOnTheScreen();
    expect(server.requests.filter((request) => request.method === "PUT")).toHaveLength(0);

    await fireEvent.changeText(screen.getByTestId("upload-limit-field"), " 100 ");
    await fireEvent.press(screen.getByTestId("upload-limit-save"));
    expect(await screen.findByText("Files can be up to 100 MB.")).toBeOnTheScreen();
    expect(screen.getByTestId("upload-notice")).toHaveTextContent(/next upload/);
    expect(server.requests.find((request) => request.method === "PUT")?.body).toEqual({
      attachmentMaxBytes: 100 << 20,
    });
  });

  it("explains a refused change", async () => {
    const server = adminServer();
    server.answer("PUT /api/v1/admin/uploads", { status: 403, body: { code: "forbidden", message: "no" } });
    await openAdmin(server);
    await fireEvent.changeText(screen.getByTestId("upload-limit-field"), "50");
    await fireEvent.press(screen.getByTestId("upload-limit-save"));
    await waitFor(() => {
      expect(screen.queryByTestId("upload-notice")).toBeNull();
    });
    expect(screen.getByText("Files can be up to 25 MB.")).toBeOnTheScreen();
  });
});
