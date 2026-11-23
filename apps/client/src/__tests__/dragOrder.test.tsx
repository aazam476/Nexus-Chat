import type { ServerDetails } from "@nexus-chat/api-client";
import { act, fireEvent, screen, waitFor } from "@testing-library/react-native";

import ChannelAdmin from "../../app/server/[id]/channels";
import RoleAdmin from "../../app/server/[id]/roles";
import { fakeServer, type FakeServer } from "../testing/fakeServer";
import { renderAs } from "../testing/messengers";
import {
  answerServer,
  bookClub,
  bookClubMembers,
  general,
  messengersInServer,
  serverId,
} from "../testing/servers";

jest.setTimeout(60_000);

const base = `/api/v1/servers/${serverId}`;

interface FakeGesture {
  enabled: boolean;
  onActivate: () => void;
  onUpdate: (event: { translationY: number }) => void;
  onDeactivate: (event: { canceled: boolean; translationY: number }) => void;
  onFinalize: () => void;
}

const gesture = (testID: string): FakeGesture =>
  (screen.getByTestId(testID).props as { gesture: FakeGesture }).gesture;

/** Lays the rows out 50 points apart, each 40 high, in the order given. */
async function layOut(testIDs: readonly string[]): Promise<void> {
  for (const [index, testID] of testIDs.entries()) {
    await fireEvent(screen.getByTestId(`${testID}-row`), "layout", {
      nativeEvent: { layout: { x: 0, y: index * 50, width: 300, height: 40 } },
    });
  }
}

/** Holds the row, moves it by `translationY`, and lets go (or the system cancels it). */
async function drag(testID: string, translationY: number, canceled = false): Promise<void> {
  const pan = gesture(testID);
  await act(async () => {
    pan.onActivate();
    pan.onUpdate({ translationY });
    pan.onDeactivate({ canceled, translationY });
    pan.onFinalize();
    await Promise.resolve();
  });
}

const shownRows = (prefix: string): string[] =>
  screen
    .getAllByTestId(new RegExp(`^${prefix}.*-row$`))
    .map((row) => (row.props as { testID: string }).testID.slice(prefix.length, -"-row".length));

/** Waits for the screen to load the server and show the row. */
const shownRow = (testID: string) => screen.findByTestId(testID, {}, { timeout: 15_000 });

const lastBody = (server: FakeServer, method: string) =>
  server.requests.filter((request) => request.method === method).at(-1)?.body;

async function render(ui: React.ReactElement, details: ServerDetails = bookClub()): Promise<FakeServer> {
  const { ada } = await messengersInServer();
  const server = fakeServer();
  answerServer(server, details);
  server.answer(`GET ${base}/members`, { status: 200, body: bookClubMembers });
  await renderAs(ui, ada, server);
  return server;
}

const channelIDs = ["voice-1", "cat-text", general].map((id) => `channel-drag-${id}`);

describe("dragging channels", () => {
  beforeEach(() => {
    (globalThis as { mockSearchParams?: object }).mockSearchParams = { id: serverId };
  });

  it("drops a channel out of its category, showing the new order at once", async () => {
    const server = await render(<ChannelAdmin />);
    server.answer(`PUT ${base}/channel-order`, { status: 204 });
    await shownRow(`channel-drag-${general}`);
    expect(shownRows("channel-drag-")).toEqual(["voice-1", "cat-text", general]);
    await layOut(channelIDs);
    await drag(`channel-drag-${general}`, -110);
    expect(shownRows("channel-drag-")).toEqual([general, "voice-1", "cat-text"]);
    await waitFor(() => {
      expect(lastBody(server, "PUT")).toEqual([{ id: general }, { id: "voice-1" }, { id: "cat-text" }]);
    });
  });

  it("sends nothing for a cancelled drag or one dropped where it began", async () => {
    const server = await render(<ChannelAdmin />);
    await shownRow(`channel-drag-${general}`);
    await layOut(channelIDs);
    await drag(`channel-drag-${general}`, -110, true);
    await drag(`channel-drag-${general}`, 10);
    expect(server.requests.some((request) => request.method === "PUT")).toBe(false);
  });

  it("goes back to the server's order when the move fails", async () => {
    const server = await render(<ChannelAdmin />);
    server.answer(`PUT ${base}/channel-order`, { status: 403, body: { code: "forbidden" } });
    await shownRow(`channel-drag-${general}`);
    await layOut(channelIDs);
    await drag(`channel-drag-${general}`, -110);
    await waitFor(() => {
      expect(shownRows("channel-drag-")).toEqual(["voice-1", "cat-text", general]);
    });
  });

  it("slides a row that moved to its new place, or moves it at once with reduced motion", async () => {
    await render(<ChannelAdmin />);
    await shownRow(`channel-drag-${general}`);
    await layOut(channelIDs);
    await layOut([...channelIDs].reverse());
    const style = () =>
      JSON.stringify((screen.getByTestId(`channel-drag-${general}-row`).props as { style: unknown }).style);
    expect(style()).toContain('"translateY":0');
    (globalThis as { mockReducedMotion?: boolean }).mockReducedMotion = true;
    await layOut(channelIDs);
    expect(style()).toContain('"translateY":0');
    (globalThis as { mockReducedMotion?: boolean }).mockReducedMotion = false;
  });
});

describe("dragging roles", () => {
  const withRoles = (): ServerDetails => {
    const details = bookClub();
    return {
      ...details,
      roles: [
        ...details.roles,
        { id: "helpers", name: "Helpers", permissions: [], position: 1, everyone: false },
        { id: "mods", name: "Mods", permissions: [], position: 2, everyone: false },
      ],
    };
  };

  it("drops a role above another, showing the new order at once", async () => {
    const server = await render(<RoleAdmin />, withRoles());
    server.answer(`PUT ${base}/role-order`, { status: 204 });
    await shownRow("role-drag-helpers");
    expect(shownRows("role-drag-")).toEqual(["mods", "helpers"]);
    expect(gesture("role-drag-helpers").enabled).toBe(true);
    await layOut(["role-drag-mods", "role-drag-helpers"]);
    await drag("role-drag-helpers", -60);
    expect(shownRows("role-drag-")).toEqual(["helpers", "mods"]);
    await waitFor(() => {
      expect(lastBody(server, "PUT")).toEqual({ roleIds: ["helpers", "mods"] });
    });
  });

  it("does not let a role be dragged past one the user may not manage", async () => {
    const details = { ...withRoles(), myRoleIds: ["mods"] };
    details.server = { ...details.server, ownerId: "someone-else" };
    const server = await render(<RoleAdmin />, details);
    await shownRow("role-drag-helpers");
    expect(gesture("role-drag-mods").enabled).toBe(false);
    await layOut(["role-drag-mods", "role-drag-helpers"]);
    await drag("role-drag-helpers", -60);
    expect(server.requests.some((request) => request.method === "PUT")).toBe(false);
  });
});
