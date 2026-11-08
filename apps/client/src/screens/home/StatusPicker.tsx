import type { UserStatus } from "@nexus-chat/api-client";
import { Button, Heading, Stack, Text } from "@nexus-chat/ui";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { useStatusChoice } from "../useStatusChoice";

const statuses: { status: UserStatus; label: string }[] = [
  { status: "online", label: "home.statusOnline" },
  { status: "idle", label: "home.statusIdle" },
  { status: "dnd", label: "home.statusDnd" },
  { status: "invisible", label: "home.statusInvisible" },
];

const presenceLabels: Partial<Record<string, string>> = {
  online: "home.statusOnline",
  idle: "home.statusIdle",
  dnd: "home.statusDnd",
  invisible: "home.statusInvisible",
  offline: "home.presenceOffline",
};

/** Props of {@link StatusPicker}. */
export interface StatusPickerProps {
  current: UserStatus;
  /** The presence others see, from the gateway; undefined until it reports. */
  presence: string | undefined;
}

/**
 * The four statuses as buttons. The chosen one is marked with a check and
 * the primary style, so the choice does not depend on color alone.
 */
export function StatusPicker({ current, presence }: StatusPickerProps): ReactNode {
  const { t } = useTranslation();
  const choice = useStatusChoice();
  return (
    <Stack gap="sm">
      <Heading level={2}>{t("home.status")}</Heading>
      {statuses.map(({ status, label }) => (
        <Button
          key={status}
          label={status === current ? `✓ ${t(label)}` : t(label)}
          variant={status === current ? "primary" : "secondary"}
          disabled={choice.busy}
          onPress={() => void choice.choose(status)}
          testID={`status-${status}`}
        />
      ))}
      {presence !== undefined && (
        <Text tone="muted" live testID="presence">
          {t("home.presence", { status: t(presenceLabels[presence] ?? "home.presenceOffline") })}
        </Text>
      )}
    </Stack>
  );
}
