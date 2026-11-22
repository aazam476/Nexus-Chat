import { fireEvent, screen, waitFor } from "@testing-library/react-native";
import { router } from "expo-router";

import Invite from "../../app/invite/[code]";
import Join from "../../app/join";
import NewServer from "../../app/new-server";
import ChannelScreen from "../../app/server/[id]/channel/[channelId]";
import ServerScreen from "../../app/server/[id]/index";
import Servers from "../../app/servers";
import { isolate } from "../i18n/bidi";
import { audiencesOf } from "../servers/audiences";
import { inviteCodeOf, inviteLink } from "../servers/serverActions";
import { fakeServer, type FakeServer } from "../testing/fakeServer";
import { renderAs } from "../testing/messengers";
import { answerServer, bookClub, general, messengersInServer, serverId } from "../testing/servers";

jest.setTimeout(60_000);

const params = (values: object): void => {
  (globalThis as { mockSearchParams?: object }).mockSearchParams = values;
};

async function serverWithBookClub(details = bookClub()): Promise<FakeServer> {
  const server = fakeServer();
  answerServer(server, details);
  return Promise.resolve(server);
}

describe("the server list", () => {
  it("lists the servers and leads to making or joining one", async () => {
    const { ada } = await messengersInServer();
    await renderAs(<Servers />, ada, await serverWithBookClub());
    expect(await screen.findByText(isolate("Book club"))).toBeOnTheScreen();
    expect(screen.getByText(isolate("We read."))).toBeOnTheScreen();
    await fireEvent.press(screen.getByTestId(`server-${serverId}`));
    expect(router.push).toHaveBeenCalledWith({ pathname: "/server/[id]", params: { id: serverId } });
    await fireEvent.press(screen.getByTestId("server-new"));
    expect(router.push).toHaveBeenCalledWith("/new-server");
    await fireEvent.press(screen.getByTestId("server-join"));
    expect(router.push).toHaveBeenCalledWith("/join");
  });

  it("says when the user is in no server", async () => {
    const { ada } = await messengersInServer();
    const server = fakeServer();
    server.answer("GET /api/v1/servers", { status: 200, body: [] });
    await renderAs(<Servers />, ada, server);
    expect(await screen.findByText(/not in any server yet/)).toBeOnTheScreen();
  });
});

describe("making a server", () => {
  it("creates it and opens it, or says why not", async () => {
    const { ada } = await messengersInServer();
    const server = await serverWithBookClub();
    server.answer("POST /api/v1/servers", { status: 422, body: { code: "invalid", message: "x" } });
    await renderAs(<NewServer />, ada, server);
    await fireEvent.changeText(screen.getByTestId("server-name"), "Book club");
    await fireEvent.changeText(screen.getByTestId("server-description"), "We read.");
    await fireEvent.press(screen.getByTestId("server-create"));
    expect(await screen.findByText(/a name is 1 to 100 characters/)).toBeOnTheScreen();

    server.answer("POST /api/v1/servers", { status: 201, body: bookClub() });
    await fireEvent.press(screen.getByTestId("server-create"));
    await waitFor(() => {
      expect(router.replace).toHaveBeenCalledWith({ pathname: "/server/[id]", params: { id: serverId } });
    });
    expect(server.requests.at(-1)?.body).toEqual({ name: "Book club", description: "We read." });
  });
});

describe("joining by invite", () => {
  const preview = { server: bookClub().server, members: 2 };

  it("looks a pasted link up and joins", async () => {
    const { ada } = await messengersInServer();
    const server = await serverWithBookClub();
    server.answer("GET /api/v1/invites/abcdefgh", { status: 200, body: preview });
    server.answer("POST /api/v1/invites/abcdefgh/join", { status: 200, body: { serverId } });
    await renderAs(<Join />, ada, server);
    await fireEvent.changeText(screen.getByTestId("invite-code"), "https://chat.example.com/invite/ABCDEFGH");
    await fireEvent.press(screen.getByTestId("invite-look-up"));
    expect(await screen.findByText(`You are invited to ${isolate("Book club")}`)).toBeOnTheScreen();
    expect(screen.getByText("2 members")).toBeOnTheScreen();
    await fireEvent.press(screen.getByTestId("invite-join"));
    await waitFor(() => {
      expect(router.replace).toHaveBeenCalledWith({ pathname: "/server/[id]", params: { id: serverId } });
    });
  });

  it("opens an invite link straight away, and says when it is not valid", async () => {
    const { ada } = await messengersInServer();
    const server = await serverWithBookClub();
    server.answer("GET /api/v1/invites/abcdefgh", { status: 200, body: preview });
    server.answer("POST /api/v1/invites/abcdefgh/join", {
      status: 404,
      body: { code: "invite_invalid", message: "x" },
    });
    params({ code: "abcdefgh" });
    await renderAs(<Invite />, ada, server);
    expect(await screen.findByText(`You are invited to ${isolate("Book club")}`)).toBeOnTheScreen();
    await fireEvent.press(screen.getByTestId("invite-join"));
    expect(await screen.findByText(/This invite is not valid/)).toBeOnTheScreen();

    await fireEvent.changeText(screen.getByTestId("invite-code"), "nope");
    await fireEvent.press(screen.getByTestId("invite-look-up"));
    await waitFor(() => {
      expect(screen.queryByTestId("invite-join")).toBeNull();
    });
  });

  it("reads codes out of links", () => {
    expect(inviteCodeOf("  https://x.example/invite/AbC123?ref=1 ")).toBe("abc123");
    expect(inviteCodeOf("abc123")).toBe("abc123");
    expect(inviteLink("https://x.example/", "abc")).toBe("https://x.example/invite/abc");
  });
});

describe("a server", () => {
  beforeEach(() => {
    params({ id: serverId });
  });

  it("shows its channels by category, and what the user may do", async () => {
    const { ada } = await messengersInServer();
    await renderAs(<ServerScreen />, ada, await serverWithBookClub());
    expect(await screen.findByText(isolate("Book club"))).toBeOnTheScreen();
    expect(screen.getByText(isolate("Text channels"))).toBeOnTheScreen();
    expect(screen.getByText(`#${isolate("general")}`)).toBeOnTheScreen();
    expect(screen.getByText(`${isolate("Lounge")} (voice)`)).toBeOnTheScreen();
    expect(screen.getByText("Other channels")).toBeOnTheScreen();
    await fireEvent.press(screen.getByTestId(`channel-${general}`));
    expect(router.push).toHaveBeenCalledWith({
      pathname: "/server/[id]/channel/[channelId]",
      params: { id: serverId, channelId: general },
    });
    await fireEvent.press(screen.getByTestId("server-invites"));
    await fireEvent.press(screen.getByTestId("server-settings"));
    expect(router.push).toHaveBeenCalledWith({ pathname: "/server/[id]/settings", params: { id: serverId } });
    // Ada owns it, so she cannot leave.
    expect(screen.queryByTestId("server-leave")).toBeNull();
  });

  it("lets a member leave after asking", async () => {
    const { ada } = await messengersInServer();
    const details = bookClub({ permissions: ["VIEW_CHANNEL"], channels: [] });
    const server = await serverWithBookClub({
      ...details,
      server: { ...details.server, ownerId: "someone" },
    });
    server.answer(`DELETE /api/v1/servers/${serverId}/members/me`, { status: 204 });
    await renderAs(<ServerScreen />, ada, server);
    expect(await screen.findByText("No channels you can see yet.")).toBeOnTheScreen();
    expect(screen.queryByTestId("server-settings")).toBeNull();
    await fireEvent.press(screen.getByTestId("server-leave"));
    await fireEvent.press(screen.getByText("Stay"));
    await fireEvent.press(screen.getByTestId("server-leave"));
    await fireEvent.press(screen.getByTestId("server-leave-confirm"));
    await waitFor(() => {
      expect(router.replace).toHaveBeenCalledWith("/servers");
    });
  });

  it("says so when the user is no longer in it", async () => {
    const { ada } = await messengersInServer();
    await renderAs(<ServerScreen />, ada, fakeServer());
    expect(await screen.findByText("You are not in this server any more.")).toBeOnTheScreen();
  });
});

describe("a channel", () => {
  beforeEach(() => {
    params({ id: serverId, channelId: general });
  });

  it("shows the channel's messages and sends in it", async () => {
    const { ada, grace } = await messengersInServer();
    const audience = audiencesOf(bookClub(), [])[0];
    if (audience !== undefined) {
      await grace.sendInChannel(audience, general, { kind: "text", text: "hello channel" });
      await ada.sync(audience.audienceId);
    }
    await renderAs(<ChannelScreen />, ada, await serverWithBookClub());
    expect(await screen.findByText(`#${isolate("general")}`)).toBeOnTheScreen();
    expect(screen.getByText(isolate("Say hello"))).toBeOnTheScreen();
    expect(await screen.findByText("hello channel")).toBeOnTheScreen();
    await fireEvent.changeText(screen.getByTestId("composer"), "hi everyone");
    await fireEvent.press(screen.getByTestId("composer-send"));
    expect(await screen.findByText("hi everyone")).toBeOnTheScreen();
  });

  it("explains a channel the user may only read, or that is gone", async () => {
    const { ada } = await messengersInServer();
    const details = bookClub();
    const readOnly = details.channels.map((channel) =>
      channel.id === general ? { ...channel, permissions: ["VIEW_CHANNEL" as const] } : channel,
    );
    await renderAs(<ChannelScreen />, ada, await serverWithBookClub({ ...details, channels: readOnly }));
    expect(await screen.findByText("You can read this channel but not write in it.")).toBeOnTheScreen();
    expect(screen.queryByTestId("composer")).toBeNull();

    params({ id: serverId, channelId: "gone" });
    await renderAs(<ChannelScreen />, ada, await serverWithBookClub());
    expect(await screen.findByText(/This channel is gone/)).toBeOnTheScreen();
  });
});
