import type { ReactNode } from "react";
import { Image, StyleSheet, Text, View } from "react-native";

import { useTheme } from "./theme";

/** Props of {@link Avatar}. */
export interface AvatarProps {
  /** The display name, for the initials and for screen readers. */
  name: string;
  /** The picture's full address; initials are shown without one. */
  imageUrl?: string;
  size?: number;
}

/** The first letter of up to two words, so "Ada Lovelace" is "AL" and an emoji name stays whole. */
export function initialsOf(name: string): string {
  const words = name
    .trim()
    .split(/\s+/)
    .filter((word) => word !== "");
  return words
    .slice(0, 2)
    .map((word) => Array.from(word)[0] ?? "")
    .join("")
    .toUpperCase();
}

/** A profile picture, or initials on the surface color when there is none. */
export function Avatar({ name, imageUrl, size = 40 }: AvatarProps): ReactNode {
  const { colors } = useTheme();
  const shape = { width: size, height: size, borderRadius: size / 2 };
  if (imageUrl !== undefined) {
    return <Image source={{ uri: imageUrl }} style={shape} accessibilityLabel={name} />;
  }
  return (
    <View
      style={[styles.initials, shape, { backgroundColor: colors.surface, borderColor: colors.border }]}
      accessible
      accessibilityLabel={name}
    >
      <Text
        style={{ color: colors.text, fontSize: size * 0.38, fontWeight: "700" }}
        maxFontSizeMultiplier={1.2}
      >
        {initialsOf(name)}
      </Text>
    </View>
  );
}

const styles = StyleSheet.create({
  initials: { alignItems: "center", justifyContent: "center", borderWidth: StyleSheet.hairlineWidth },
});
