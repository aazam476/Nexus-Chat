import { Button, Stack, Text } from "@nexus-chat/ui";
import { useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";

/** Leaving the group, after a second press that confirms it. */
export function LeaveGroup({ leave, busy }: { leave: () => void; busy: boolean }): ReactNode {
  const { t } = useTranslation();
  const [asking, setAsking] = useState(false);
  return (
    <Stack gap="sm">
      {asking ? <Text live>{t("groups.leaveConfirm")}</Text> : null}
      <Button
        label={t("groups.leave")}
        variant="danger"
        busy={busy}
        onPress={() => {
          if (asking) {
            leave();
          } else {
            setAsking(true);
          }
        }}
        testID="group-leave"
      />
    </Stack>
  );
}
