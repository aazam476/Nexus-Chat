import {
  shouldReportTyping,
  startedTyping,
  stillTyping,
  stoppedTyping,
  typingRepeatMs,
  typingShownMs,
  type Typists,
} from "../messaging/typing";

describe("typing state", () => {
  it("shows a member for a few seconds after each typing_start", () => {
    let typists: Typists = new Map();
    typists = startedTyping(typists, "grace", 1000);
    typists = startedTyping(typists, "alan", 2000);
    expect(stillTyping(typists, 1000 + typingShownMs - 1)).toEqual(["grace", "alan"]);
    expect(stillTyping(typists, 1000 + typingShownMs)).toEqual(["alan"]);
    typists = startedTyping(typists, "grace", 9000);
    expect(stillTyping(typists, 9500)).toEqual(["grace", "alan"]);
  });

  it("stops a member at once, and leaves the rest alone", () => {
    const typists = startedTyping(new Map(), "grace", 0);
    expect(stillTyping(stoppedTyping(typists, "grace"), 1)).toEqual([]);
    expect(stoppedTyping(typists, "nobody")).toBe(typists);
  });

  it("repeats this device's typing only every few seconds", () => {
    expect(shouldReportTyping(undefined, 0)).toBe(true);
    expect(shouldReportTyping(1000, 1000 + typingRepeatMs - 1)).toBe(false);
    expect(shouldReportTyping(1000, 1000 + typingRepeatMs)).toBe(true);
  });
});
