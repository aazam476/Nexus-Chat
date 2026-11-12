import { screen } from "@testing-library/react-native";

import SelfTest from "../../app/self-test";
import { runVectors } from "../selfTest/vectorRun";
import { renderWithApp } from "../testing/renderWithApp";

jest.mock("../selfTest/vectorRun", () => ({ runVectors: jest.fn() }));

type Report = (progress: { done: number; total: number; failures: string[] }) => void;

describe("the self-test screen", () => {
  afterEach(() => {
    delete process.env.EXPO_PUBLIC_NEXUS_VECTORS_URL;
  });

  it("does not exist outside CI's test build", async () => {
    await renderWithApp(<SelfTest />);
    expect(screen.getByText("redirect:/")).toBeOnTheScreen();
    expect(runVectors).not.toHaveBeenCalled();
  });

  it("shows progress, then that every case passed", async () => {
    process.env.EXPO_PUBLIC_NEXUS_VECTORS_URL = "http://vectors";
    jest.mocked(runVectors).mockImplementationOnce((_url: string, report: Report) => {
      report({ done: 1, total: 1, failures: [] });
      return Promise.resolve({ done: 1, total: 1, failures: [] });
    });
    await renderWithApp(<SelfTest />);
    expect(await screen.findByTestId("self-test-passed")).toHaveTextContent("1 of 1 cases run, 0 failed");
  });

  it("names each failed case", async () => {
    process.env.EXPO_PUBLIC_NEXUS_VECTORS_URL = "http://vectors";
    jest.mocked(runVectors).mockImplementationOnce((_url: string, report: Report) => {
      report({ done: 1, total: 2, failures: ["case: differs"] });
      report({ done: 2, total: 2, failures: ["case: differs"] });
      return Promise.resolve({ done: 2, total: 2, failures: ["case: differs"] });
    });
    await renderWithApp(<SelfTest />);
    expect(await screen.findByTestId("self-test-failed")).toHaveTextContent("case: differs");
    expect(screen.getByTestId("self-test-progress")).toHaveTextContent("2 of 2 cases run, 1 failed");
  });

  it("says when the vectors could not be run", async () => {
    process.env.EXPO_PUBLIC_NEXUS_VECTORS_URL = "http://vectors";
    jest.mocked(runVectors).mockReturnValueOnce(new Promise(() => undefined));
    await renderWithApp(<SelfTest />);
    expect(screen.getByText("Fetching the test vectors…")).toBeOnTheScreen();
    await screen.unmount();

    jest.mocked(runVectors).mockRejectedValueOnce(new Error("offline"));
    await renderWithApp(<SelfTest />);
    expect(await screen.findByTestId("self-test-failed")).toHaveTextContent("Error: offline");
  });
});
