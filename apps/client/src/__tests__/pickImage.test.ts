import { launchImageLibraryAsync } from "expo-image-picker";

import { pickSquareImage } from "../platform/pickImage";

jest.mock("expo-image-picker", () => ({ launchImageLibraryAsync: jest.fn() }));

describe("picking a square image", () => {
  it("returns the chosen picture's bytes", async () => {
    jest.mocked(launchImageLibraryAsync).mockResolvedValueOnce({
      canceled: false,
      assets: [{ uri: "data:image/png;base64,iVBORw==", width: 1, height: 1 }],
    });
    const image = await pickSquareImage();
    expect(image).toBeInstanceOf(Blob);
    expect(launchImageLibraryAsync).toHaveBeenCalledWith(
      expect.objectContaining({ allowsEditing: true, aspect: [1, 1] }),
    );
  });

  it("returns nothing when the user cancels", async () => {
    jest.mocked(launchImageLibraryAsync).mockResolvedValueOnce({ canceled: true, assets: null });
    expect(await pickSquareImage()).toBeUndefined();
  });
});
