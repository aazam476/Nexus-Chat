import { fireEvent, render, screen } from "@testing-library/react-native";
import type { ReactElement, ReactNode } from "react";
import { Text } from "react-native";
import { SafeAreaProvider } from "react-native-safe-area-context";

import { Avatar, initialsOf } from "./Avatar";
import { Badge } from "./Badge";
import { ConversationLayout } from "./ConversationLayout";
import { ListRow } from "./ListRow";
import { ThemeProvider } from "./theme";

function themed(node: ReactNode): ReactElement {
  return (
    <SafeAreaProvider
      initialMetrics={{
        frame: { x: 0, y: 0, width: 400, height: 800 },
        insets: { top: 0, left: 0, right: 0, bottom: 0 },
      }}
    >
      <ThemeProvider preference="dark">{node}</ThemeProvider>
    </SafeAreaProvider>
  );
}

describe("Badge", () => {
  it("shows a count with its meaning, caps at 99+, and hides at zero", async () => {
    await render(themed(<Badge count={3} label="3 unread messages" />));
    expect(screen.getByLabelText("3 unread messages")).toBeOnTheScreen();
    expect(screen.getByText("3")).toBeOnTheScreen();

    await render(themed(<Badge count={12_345} label="12345 unread messages" />));
    expect(screen.getByText("99+")).toBeOnTheScreen();

    await render(themed(<Badge count={0} label="none" />));
    expect(screen.queryByLabelText("none")).toBeNull();
  });
});

describe("Avatar", () => {
  it("shows initials without a picture, and the picture with one", async () => {
    await render(themed(<Avatar name="Ada Lovelace" />));
    expect(screen.getByText("AL")).toBeOnTheScreen();

    await render(themed(<Avatar name="Grace" imageUrl="https://example.com/a.png" size={24} />));
    expect(screen.getByLabelText("Grace")).toBeOnTheScreen();
    expect(screen.queryByText("G")).toBeNull();
  });

  it("takes whole characters for initials", () => {
    expect(initialsOf("  ada   byron lovelace ")).toBe("AB");
    expect(initialsOf("🦊 Fox")).toBe("🦊F");
    expect(initialsOf("")).toBe("");
  });
});

describe("ListRow", () => {
  it("is a named button when pressable, and shows its parts", async () => {
    const onPress = jest.fn();
    await render(
      themed(
        <ListRow
          title="Grace"
          subtitle="See you soon"
          emphasized
          leading={<Text>L</Text>}
          trailing={<Text>T</Text>}
          onPress={onPress}
          accessibilityLabel="Grace, 2 unread"
        />,
      ),
    );

    const row = screen.getByRole("button", { name: "Grace, 2 unread" });
    await fireEvent(row, "pressIn");
    await fireEvent(row, "focus");
    await fireEvent.press(row);
    await fireEvent(row, "pressOut");
    await fireEvent(row, "blur");
    expect(onPress).toHaveBeenCalledTimes(1);
    expect(screen.getByText("See you soon")).toBeOnTheScreen();
    expect(screen.getByText("L")).toBeOnTheScreen();
    expect(screen.getByText("T")).toBeOnTheScreen();
  });

  it("is plain content without onPress, named by what it shows when pressable", async () => {
    await render(themed(<ListRow title="Just text" testID="row" />));
    expect(screen.queryByRole("button")).toBeNull();
    expect(screen.getByText("Just text")).toBeOnTheScreen();

    await render(themed(<ListRow title="Named" subtitle="and more" onPress={() => undefined} />));
    expect(screen.getByRole("button", { name: /Named.*and more/ })).toBeOnTheScreen();
  });
});

describe("ConversationLayout", () => {
  it("puts the header, list and footer in one main region", async () => {
    await render(
      themed(
        <ConversationLayout header={<Text>Header</Text>} footer={<Text>Composer</Text>}>
          <Text>Messages</Text>
        </ConversationLayout>,
      ),
    );

    expect(JSON.stringify(screen.toJSON())).toContain(`"role":"main"`);
    for (const part of ["Header", "Messages", "Composer"]) {
      expect(screen.getByText(part)).toBeOnTheScreen();
    }
  });
});
