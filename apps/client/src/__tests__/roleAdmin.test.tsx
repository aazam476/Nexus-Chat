import type { ServerDetails } from "@nexus-chat/api-client";
import { fireEvent, screen, waitFor } from "@testing-library/react-native";
import { router } from "expo-router";

import Member from "../../app/server/[id]/member/[userId]";
import Members from "../../app/server/[id]/members";
import EditRole from "../../app/server/[id]/role/[roleId]";
import RoleAdmin from "../../app/server/[id]/roles";
import { isolate } from "../i18n/bidi";
import { fakeServer, type FakeServer } from "../testing/fakeServer";
import { renderAs } from "../testing/messengers";
import { answerServer, bookClub, bookClubMembers, messengersInServer, serverId } from "../testing/servers";

jest.setTimeout(60_000);

const base = `/api/v1/servers/${serverId}`;
const grace = bookClubMembers[1]?.user.id ?? "";

/** Book club with Mods above Helpers; Grace is a Helper. */
function withRoles(extra: Partial<ServerDetails> = {}): ServerDetails {
  const details = bookClub();
  return {
    ...details,
    roles: [
      ...details.roles,
      {
        id: "helpers",
        name: "Helpers",
        permissions: ["KICK_MEMBERS"],
        position: 1,
        everyone: false,
        color: 0x30a46c,
      },
      { id: "mods", name: "Mods", permissions: [], position: 2, everyone: false },
    ],
    ...extra,
  };
}

async function render(ui: React.ReactElement, details = withRoles()): Promise<FakeServer> {
  const { ada } = await messengersInServer();
  const server = fakeServer();
  answerServer(server, details);
  server.answer(`GET ${base}/members`, {
    status: 200,
    body: bookClubMembers.map((member) =>
      member.user.id === grace ? { ...member, roleIds: ["helpers"] } : member,
    ),
  });
  await renderAs(ui, ada, server);
  return server;
}

const lastBody = (server: FakeServer, method: string) =>
  server.requests.filter((request) => request.method === method).at(-1)?.body;
const params = (values: object): void => {
  (globalThis as { mockSearchParams?: object }).mockSearchParams = values;
};

describe("managing roles", () => {
  beforeEach(() => {
    params({ id: serverId, roleId: "helpers", userId: grace });
  });

  it("lists roles highest first, moves them, and adds one", async () => {
    const server = await render(<RoleAdmin />);
    server.answer(`PUT ${base}/role-order`, { status: 204 });
    server.answer(`POST ${base}/roles`, { status: 201, body: { id: "new-role" } });
    expect(await screen.findByText("@everyone: what every member may do")).toBeOnTheScreen();
    expect(screen.getAllByText("1 member")).toHaveLength(1);
    await fireEvent.press(screen.getByTestId("role-up-helpers"));
    await waitFor(() => {
      expect(lastBody(server, "PUT")).toEqual({ roleIds: ["helpers", "mods"] });
    });
    await fireEvent.press(screen.getByTestId("role-new"));
    await waitFor(() => {
      expect(router.push).toHaveBeenCalledWith({
        pathname: "/server/[id]/role/[roleId]",
        params: { id: serverId, roleId: "new-role" },
      });
    });
    expect(lastBody(server, "POST")).toEqual({ name: "New role", permissions: [] });
    await fireEvent.press(screen.getByTestId("role-edit-mods"));
    expect(router.push).toHaveBeenCalledWith({
      pathname: "/server/[id]/role/[roleId]",
      params: { id: serverId, roleId: "mods" },
    });
  });

  it("edits a role's name, color and permissions, and deletes it", async () => {
    const server = await render(<EditRole />);
    server.answer(`PATCH ${base}/roles/helpers`, { status: 204 });
    server.answer(`DELETE ${base}/roles/helpers`, { status: 204 });
    await fireEvent.changeText(await screen.findByTestId("role-name"), "Helpers team");
    expect(screen.getByTestId("role-color-4")).toHaveTextContent("✓ Green");
    await fireEvent.press(screen.getByTestId("role-color-0"));
    // Ada lacks KICK_MEMBERS herself, so the role keeps it as it is.
    expect(screen.getByTestId("role-perm-KICK_MEMBERS")).toBeDisabled();
    await fireEvent.press(screen.getByTestId("role-perm-SEND_MESSAGES"));
    await fireEvent.press(screen.getByTestId("role-save"));
    await waitFor(() => {
      expect(lastBody(server, "PATCH")).toEqual({
        name: "Helpers team",
        permissions: ["KICK_MEMBERS", "SEND_MESSAGES"],
      });
    });
    await fireEvent.press(screen.getByTestId("role-color-6"));
    await fireEvent.press(screen.getByTestId("role-save"));
    await waitFor(() => {
      expect(lastBody(server, "PATCH")).toMatchObject({ color: 0x0090ff });
    });
    await fireEvent.press(screen.getByTestId("role-delete"));
    await fireEvent.press(screen.getByTestId("role-delete-confirm"));
    await waitFor(() => {
      expect(server.requests.some((request) => request.method === "DELETE")).toBe(true);
    });
  });

  it("keeps @everyone's name and never deletes it", async () => {
    params({ id: serverId, roleId: "role-everyone" });
    await render(<EditRole />);
    expect(await screen.findByTestId("role-save")).toBeOnTheScreen();
    expect(screen.queryByTestId("role-name")).toBeNull();
    expect(screen.queryByTestId("role-delete")).toBeNull();
  });
});

describe("members", () => {
  beforeEach(() => {
    params({ id: serverId, userId: grace });
  });

  it("lists who is in the server and with which roles", async () => {
    await render(<Members />);
    expect(await screen.findByText("Owner")).toBeOnTheScreen();
    expect(screen.getByText(isolate("Helpers"))).toBeOnTheScreen();
    await fireEvent.press(screen.getByTestId(`member-${grace}`));
    expect(router.push).toHaveBeenCalledWith({
      pathname: "/server/[id]/member/[userId]",
      params: { id: serverId, userId: grace },
    });
  });

  it("gives and takes roles below the user's own", async () => {
    const server = await render(<Member />);
    server.answer(`PUT ${base}/members/${grace}/roles/mods`, { status: 204 });
    server.answer(`DELETE ${base}/members/${grace}/roles/helpers`, { status: 204 });
    expect(await screen.findByText(isolate("Grace"))).toBeOnTheScreen();
    await fireEvent.press(screen.getByTestId("member-role-mods"));
    await fireEvent.press(screen.getByTestId("member-role-helpers"));
    await waitFor(() => {
      expect(
        server.requests
          .filter((request) => request.path.includes("/roles/"))
          .map((request) => request.method),
      ).toEqual(["PUT", "DELETE"]);
    });
  });

  it("only shows the roles the user may not give", async () => {
    const details = withRoles({ myRoleIds: ["helpers"], permissions: ["MANAGE_ROLES"] });
    await render(<Member />, { ...details, server: { ...details.server, ownerId: "someone" } });
    expect(await screen.findByText(`Has ${isolate("Helpers")}`)).toBeOnTheScreen();
    expect(screen.getByText(`Does not have ${isolate("Mods")}`)).toBeOnTheScreen();
    expect(screen.queryByTestId("member-role-mods")).toBeNull();
  });
});
