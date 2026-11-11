import { fireEvent, screen, waitFor } from "@testing-library/react-native";
import { router } from "expo-router";

import Home from "../../app/home";
import Recovery from "../../app/recovery";
import Restore from "../../app/restore";
import { answersMatch, pickPositions } from "../messaging/phraseCheck";
import type { Messenger } from "../messaging/messenger";
import { messengersInDm } from "../testing/messengers";
import { fakeServer } from "../testing/fakeServer";
import { ada, fakeConnection, fakeMessaging, renderWithApp } from "../testing/renderWithApp";

jest.setTimeout(60_000);

const phrase = Array.from({ length: 24 }, (_, index) => `word${String(index + 1)}`).join(" ");
let messenger: Messenger;

beforeAll(async () => {
  ({ ada: messenger } = await messengersInDm());
});

afterEach(() => {
  jest.restoreAllMocks();
});

const signedIn = () => fakeConnection(fakeServer(), { user: ada });

describe("checking the recovery phrase", () => {
  it("picks distinct positions in order and compares words loosely", () => {
    const values = [0.9, 0.1, 0.1, 0.5];
    expect(pickPositions(24, () => values.shift() ?? 0)).toEqual([2, 12, 21]);
    expect(pickPositions(2)).toEqual([0, 1]);
    expect(answersMatch("a b c", [0, 2], [" A ", "c"])).toBe(true);
    expect(answersMatch("a b c", [0, 2], ["a", "b"])).toBe(false);
    expect(answersMatch("a b c", [0, 2], ["a"])).toBe(false);
    expect(answersMatch("a b", [5], ["a"])).toBe(false);
  });
});

describe("the recovery phrase screen", () => {
  it("shows the words, then confirms them by asking for three", async () => {
    // Words 1, 13 and 24, every time the check is shown.
    const draws = [0, 0.5, 0.99];
    let draw = 0;
    jest.spyOn(Math, "random").mockImplementation(() => draws[draw++ % draws.length] ?? 0);
    const messaging = fakeMessaging({ status: "recovery-phrase", phrase, messenger });
    await renderWithApp(<Recovery />, signedIn(), messaging);
    expect(screen.getByText("1. word1")).toBeOnTheScreen();
    expect(screen.getByText("24. word24")).toBeOnTheScreen();

    await fireEvent.press(screen.getByTestId("recovery-wrote"));
    await fireEvent.press(screen.getByText("Show the phrase again"));
    await fireEvent.press(screen.getByTestId("recovery-wrote"));
    await fireEvent.changeText(screen.getByTestId("recovery-word-0"), "word1");
    await fireEvent.changeText(screen.getByTestId("recovery-word-1"), "wrong");
    await fireEvent.changeText(screen.getByTestId("recovery-word-2"), "word24");
    await fireEvent.press(screen.getByTestId("recovery-confirm"));
    expect(screen.getByText(/don't match your phrase/)).toBeOnTheScreen();
    expect(messaging.confirmPhrase).not.toHaveBeenCalled();

    await fireEvent.changeText(screen.getByTestId("recovery-word-1"), "WORD13");
    expect(screen.queryByText(/don't match your phrase/)).toBeNull();
    await fireEvent.press(screen.getByTestId("recovery-confirm"));
    await waitFor(() => {
      expect(messaging.confirmPhrase).toHaveBeenCalled();
    });
  });

  it("goes home when there is no phrase to show", async () => {
    await renderWithApp(<Recovery />, signedIn(), fakeMessaging({ status: "ready", messenger }));
    expect(router.replace).toHaveBeenCalledWith("/home");
  });
});

describe("the restore screen", () => {
  it("restores from the phrase and says when it is wrong", async () => {
    const restore = jest.fn().mockResolvedValueOnce(false).mockResolvedValueOnce(true);
    const connection = signedIn();
    await renderWithApp(<Restore />, connection, fakeMessaging({ status: "needs-restore", restore }));
    await fireEvent.changeText(screen.getByTestId("restore-phrase"), "not it");
    await fireEvent.press(screen.getByTestId("restore-submit"));
    expect(await screen.findByText(/doesn't open your backup/)).toBeOnTheScreen();
    await fireEvent.changeText(screen.getByTestId("restore-phrase"), phrase);
    expect(screen.queryByText(/doesn't open your backup/)).toBeNull();
    await fireEvent.press(screen.getByTestId("restore-submit"));
    await waitFor(() => {
      expect(restore).toHaveBeenLastCalledWith(phrase);
    });
    await fireEvent.press(screen.getByText("Log out"));
    expect(connection.signOut).toHaveBeenCalled();
  });

  it("goes home when nothing needs restoring", async () => {
    await renderWithApp(<Restore />, signedIn(), fakeMessaging({ status: "off" }));
    expect(router.replace).toHaveBeenCalledWith("/home");
  });
});

describe("messaging on the home screen", () => {
  it("leads to messages and friends once ready", async () => {
    await renderWithApp(<Home />, signedIn(), fakeMessaging({ status: "ready", messenger }));
    await fireEvent.press(screen.getByTestId("open-messages"));
    expect(router.push).toHaveBeenCalledWith("/conversations");
    await fireEvent.press(screen.getByTestId("open-friends"));
    expect(router.push).toHaveBeenCalledWith("/friends");
  });

  it("says while encryption is being set up", async () => {
    await renderWithApp(<Home />, signedIn(), fakeMessaging({ status: "starting" }));
    expect(screen.getByText(/Setting up encryption/)).toBeOnTheScreen();
  });

  it("offers to try again when setup failed", async () => {
    const messaging = fakeMessaging({ status: "failed" });
    await renderWithApp(<Home />, signedIn(), messaging);
    await fireEvent.press(screen.getByText("Try again"));
    expect(messaging.retry).toHaveBeenCalled();
  });

  it.each([
    ["recovery-phrase", "/recovery"],
    ["needs-restore", "/restore"],
  ] as const)("leads a user who must act (%s) to %s", async (status, route) => {
    const messaging =
      status === "recovery-phrase" ? fakeMessaging({ status, phrase, messenger }) : fakeMessaging({ status });
    await renderWithApp(<Home />, signedIn(), messaging);
    await fireEvent.press(screen.getByTestId("messaging-next-step"));
    expect(router.push).toHaveBeenCalledWith(route);
  });
});
