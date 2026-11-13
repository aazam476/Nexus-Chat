import { fireEvent, screen, waitFor } from "@testing-library/react-native";

import Home from "../../app/home";
import { fakeServer } from "../testing/fakeServer";
import { ada, fakeConnection, renderWithApp } from "../testing/renderWithApp";

describe("choosing how photos are sent", () => {
  it("starts on compressing over mobile data and remembers another choice", async () => {
    const connection = fakeConnection(fakeServer(), { user: ada });
    await renderWithApp(<Home />, connection);
    expect(screen.getByTestId("media-quality-auto")).toHaveTextContent("✓ Compress on mobile data");
    await fireEvent.press(screen.getByTestId("media-quality-original"));
    expect(screen.getByTestId("media-quality-original")).toHaveTextContent("✓ Always send originals");
    await waitFor(async () => {
      expect(await connection.storage.get("nexus.mediaQuality")).toBe("original");
    });
    await renderWithApp(<Home />, connection);
    expect(await screen.findByText("✓ Always send originals")).toBeOnTheScreen();
  });
});
