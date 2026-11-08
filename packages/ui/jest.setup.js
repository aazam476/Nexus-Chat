// Reanimated needs its native UI-thread runtime, which Jest does not have,
// and its bundled mock loads native code too. The components use only
// three things from it, so they are replaced with plain equivalents here.
// Tests turn reduced motion on with globalThis.mockReducedMotion = true.
jest.mock("react-native-reanimated", () => {
  const { Text, View } = jest.requireActual("react-native");
  return {
    __esModule: true,
    default: { View, Text },
    cubicBezier: (x1, y1, x2, y2) => `cubic-bezier(${x1}, ${y1}, ${x2}, ${y2})`,
    useReducedMotion: () => globalThis.mockReducedMotion === true,
  };
});
