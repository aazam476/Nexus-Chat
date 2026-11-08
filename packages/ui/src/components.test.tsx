import { fireEvent, render, screen } from "@testing-library/react-native";
import * as ReactNative from "react-native";
import { Linking } from "react-native";
import type { ReactElement, ReactNode } from "react";
import { SafeAreaProvider } from "react-native-safe-area-context";

import { Banner } from "./Banner";
import { Button } from "./Button";
import { Heading } from "./Heading";
import { Link } from "./Link";
import { Screen } from "./Screen";
import { Stack } from "./Stack";
import { Text } from "./Text";
import { TextField } from "./TextField";
import { ThemeProvider, useTheme, type ThemePreference } from "./theme";
import { colors } from "./tokens";

function themed(node: ReactNode, preference: ThemePreference = "light"): ReactElement {
  return (
    <SafeAreaProvider
      initialMetrics={{
        frame: { x: 0, y: 0, width: 400, height: 800 },
        insets: { top: 0, left: 0, right: 0, bottom: 0 },
      }}
    >
      <ThemeProvider preference={preference}>{node}</ThemeProvider>
    </SafeAreaProvider>
  );
}

describe("Button", () => {
  it("is a named button that reports presses", async () => {
    const onPress = jest.fn();
    await render(themed(<Button label="Log in" onPress={onPress} />));

    await fireEvent.press(screen.getByRole("button", { name: "Log in" }));

    expect(onPress).toHaveBeenCalledTimes(1);
  });

  it("ignores presses while busy or disabled, and says so", async () => {
    const onPress = jest.fn();
    await render(
      themed(
        <Stack>
          <Button label="Saving" busy onPress={onPress} />
          <Button label="Off" disabled variant="secondary" onPress={onPress} />
          <Button label="Delete" variant="danger" onPress={onPress} />
        </Stack>,
      ),
    );

    await fireEvent.press(screen.getByRole("button", { name: "Saving" }));
    await fireEvent.press(screen.getByRole("button", { name: "Off" }));

    expect(onPress).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: "Saving" })).toBeBusy();
    expect(screen.getByRole("button", { name: "Saving" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Delete" })).toBeEnabled();
  });

  it("shows press and focus states", async () => {
    await render(themed(<Button label="Go" onPress={jest.fn()} />));
    const button = screen.getByRole("button", { name: "Go" });

    await fireEvent(button, "pressIn");
    await fireEvent(button, "focus");
    await fireEvent(button, "pressOut");
    await fireEvent(button, "blur");

    expect(button).toBeOnTheScreen();
  });
});

describe("TextField", () => {
  it("links its label, hint and error to the input", async () => {
    const onChangeText = jest.fn();
    await render(
      themed(
        <TextField
          label="Email"
          value=""
          onChangeText={onChangeText}
          hint="We never show it"
          error="Required"
          testID="email"
        />,
      ),
    );
    const input = screen.getByLabelText("Email");

    await fireEvent.changeText(input, "a@example.com");
    await fireEvent(input, "focus");
    await fireEvent(input, "blur");

    expect(onChangeText).toHaveBeenCalledWith("a@example.com");
    expect(input.props["aria-describedby"]).toBe("email-hint email-error");
    expect(input.props["aria-invalid"]).toBe(true);
    expect(screen.getByText("Required")).toBeOnTheScreen();
  });

  it("has no description without hint or error", async () => {
    await render(themed(<TextField label="Name" value="Ada" onChangeText={jest.fn()} secure />));

    expect(screen.getByLabelText("Name").props["aria-describedby"]).toBeUndefined();
  });

  it("names its hint after the label when it has no test ID", async () => {
    await render(themed(<TextField label="Code" value="" onChangeText={jest.fn()} hint="Six digits" />));

    expect(screen.getByLabelText("Code").props["aria-describedby"]).toBe("Code-hint");
  });

  it("grows to the given number of lines for multi-line text", async () => {
    await render(themed(<TextField label="Certificate" value="" onChangeText={jest.fn()} lines={6} />));

    const input = screen.getByLabelText("Certificate");
    expect(input.props.multiline).toBe(true);
    expect(input.props.numberOfLines).toBe(6);
    expect(screen.getByLabelText("Certificate")).toHaveStyle({ minHeight: 144 });
  });
});

describe("Link", () => {
  it("opens its address", async () => {
    const openURL = jest.spyOn(Linking, "openURL").mockResolvedValue(undefined);
    await render(themed(<Link label="Learn more" href="https://example.com" />));
    const link = screen.getByRole("link", { name: "Learn more" });

    await fireEvent(link, "focus");
    await fireEvent.press(link);
    await fireEvent(link, "blur");

    expect(openURL).toHaveBeenCalledWith("https://example.com");
  });

  it("can navigate instead, or do nothing without a target", async () => {
    const onPress = jest.fn();
    await render(
      themed(
        <Stack>
          <Link label="Sign up" onPress={onPress} />
          <Link label="Nowhere" />
        </Stack>,
      ),
    );

    await fireEvent.press(screen.getByRole("link", { name: "Sign up" }));
    await fireEvent.press(screen.getByRole("link", { name: "Nowhere" }));

    expect(onPress).toHaveBeenCalledTimes(1);
  });
});

describe("layout and text", () => {
  it("renders a screen with headings, text tones and a banner", async () => {
    await render(
      themed(
        <Screen>
          <Heading>Welcome</Heading>
          <Heading level={2}>Details</Heading>
          <Text tone="muted">Quiet</Text>
          <Text tone="danger" variant="small">
            Loud
          </Text>
          <Banner title="Warning: not secure">
            <Text>Use HTTPS.</Text>
          </Banner>
        </Screen>,
      ),
    );

    expect(screen.getByRole("heading", { name: "Welcome" })).toBeOnTheScreen();
    expect(screen.getByLabelText("Warning: not secure")).toBeOnTheScreen();
  });
});

describe("ThemeProvider", () => {
  function SchemeName(): ReactNode {
    const theme = useTheme();
    return <Text>{`${theme.scheme} ${theme.colors.background}`}</Text>;
  }

  it.each([
    ["light", "light"],
    ["dark", "dark"],
    ["system", "light"],
  ] as const)("with preference %s uses the %s theme", async (preference, scheme) => {
    await render(themed(<SchemeName />, preference));

    expect(screen.getByText(`${scheme} ${colors[scheme].background}`)).toBeOnTheScreen();
  });
});

describe("reduced motion", () => {
  afterEach(() => {
    (globalThis as { mockReducedMotion?: boolean }).mockReducedMotion = false;
  });

  it("dims instead of shrinking a pressed button", async () => {
    (globalThis as { mockReducedMotion?: boolean }).mockReducedMotion = true;
    await render(themed(<Button label="Calm" onPress={jest.fn()} />));
    const button = screen.getByRole("button", { name: "Calm" });

    await fireEvent(button, "pressIn");

    expect(JSON.stringify(screen.toJSON())).toContain('"opacity":0.8');
  });
});

describe("following the system", () => {
  it("uses the dark theme when the system is dark", async () => {
    jest.spyOn(ReactNative, "useColorScheme").mockReturnValue("dark");
    function Scheme(): ReactNode {
      return <Text>{useTheme().scheme}</Text>;
    }

    await render(themed(<Scheme />, "system"));

    expect(screen.getByText("dark")).toBeOnTheScreen();
  });
});
