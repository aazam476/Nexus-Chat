import { Avatar, minimumTouchTarget, spacing, Text, useTheme } from "@nexus-chat/ui";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Text as NativeText, Pressable, StyleSheet, View } from "react-native";

/** Props of {@link ChoiceRow}. */
export interface ChoiceRowProps {
  name: string;
  detail: string;
  imageUrl?: string | undefined;
  chosen: boolean;
  /** Off once the group is full, for people not chosen yet. */
  enabled: boolean;
  onToggle: () => void;
  testID?: string;
}

/** The box at the end of the row, filled with a check mark when chosen. */
function Checkbox({ chosen }: { chosen: boolean }): ReactNode {
  const { t } = useTranslation();
  const { colors } = useTheme();
  const fill = chosen ? colors.primary : "transparent";
  return (
    <View
      style={[styles.box, { borderColor: chosen ? colors.primary : colors.border, backgroundColor: fill }]}
    >
      {chosen ? <NativeText style={{ color: colors.onPrimary }}>{t("groups.checkMark")}</NativeText> : null}
    </View>
  );
}

/**
 * A person to pick, as a checkbox row. Picking shows at once as a check
 * mark and a tint, with no animation: it is pressed many times in a row.
 */
export function ChoiceRow({
  name,
  detail,
  imageUrl,
  chosen,
  enabled,
  onToggle,
  testID,
}: ChoiceRowProps): ReactNode {
  const { t } = useTranslation();
  const { colors } = useTheme();
  return (
    <Pressable
      role="checkbox"
      accessibilityState={{ checked: chosen, disabled: !enabled }}
      accessibilityLabel={t("groups.member", {
        name,
        state: chosen ? t("groups.chosen") : t("groups.notChosen"),
      })}
      disabled={!enabled}
      onPress={onToggle}
      testID={testID}
      style={({ pressed }) => [
        styles.row,
        { backgroundColor: pressed || chosen ? colors.surface : "transparent", opacity: enabled ? 1 : 0.5 },
      ]}
    >
      <Avatar name={name} {...(imageUrl === undefined ? {} : { imageUrl })} />
      <View style={styles.text}>
        <Text>{name}</Text>
        <Text variant="small" tone="muted">
          {detail}
        </Text>
      </View>
      <Checkbox chosen={chosen} />
    </Pressable>
  );
}

const styles = StyleSheet.create({
  row: {
    flexDirection: "row",
    alignItems: "center",
    gap: spacing.md,
    minHeight: minimumTouchTarget,
    padding: spacing.sm,
    borderRadius: 8,
  },
  text: { flex: 1, minWidth: 0 },
  box: {
    width: 24,
    height: 24,
    borderRadius: 6,
    borderWidth: 2,
    alignItems: "center",
    justifyContent: "center",
  },
});
