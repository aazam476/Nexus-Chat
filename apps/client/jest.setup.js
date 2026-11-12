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

// Navigation is checked by what the screens ask for, not by a real router.
// Tests set a route's parameters with globalThis.mockSearchParams.
jest.mock("expo-router", () => {
  const { createElement } = jest.requireActual("react");
  const { Text } = jest.requireActual("react-native");
  return {
    router: { replace: jest.fn(), push: jest.fn(), back: jest.fn() },
    useLocalSearchParams: () => globalThis.mockSearchParams ?? {},
    Redirect: ({ href }) => createElement(Text, null, `redirect:${href}`),
    Stack: () => null,
  };
});

// op-sqlite is native code; screens get the database through the messaging
// environment, and the database tests drive a fake of it.
jest.mock("@op-engineering/op-sqlite", () => ({ open: jest.fn() }));
// react-native-quick-crypto is native code; its own mock forwards to Node's crypto.
jest.mock("react-native-quick-crypto", () => jest.requireActual("react-native-quick-crypto/jest"));
