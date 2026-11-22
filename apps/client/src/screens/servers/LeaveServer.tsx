import { Button, Stack, Text } from "@nexus-chat/ui";
import { router } from "expo-router";
import { useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { useConnection } from "../../connection/ConnectionContext";
import { isolate } from "../../i18n/bidi";
import { leave, serverErrorText } from "../../servers/serverActions";

/** Props of {@link ConfirmLeave}. */
interface ConfirmLeaveProps {
  serverId: string;
  name: string;
  cancel: () => void;
}

/** The question asked before leaving, and its answers. */
function ConfirmLeave({ serverId, name, cancel }: ConfirmLeaveProps): ReactNode {
  const { t } = useTranslation();
  const { client } = useConnection();
  const [error, setError] = useState<string>();
  const confirm = (): void => {
    if (client !== undefined) {
      void leave(client, serverId).then((result) => {
        if (result.ok) {
          router.replace("/servers");
        } else {
          setError(serverErrorText(result.code, t));
        }
      });
    }
  };
  return (
    <Stack gap="sm">
      <Text live>{t("servers.leaveConfirm", { name: isolate(name) })}</Text>
      {error === undefined ? null : <Text tone="danger">{error}</Text>}
      <Button
        label={t("servers.leaveYes")}
        variant="danger"
        onPress={confirm}
        testID="server-leave-confirm"
      />
      <Button label={t("servers.leaveNo")} variant="secondary" onPress={cancel} />
    </Stack>
  );
}

/** Leaving the server, asked once more since coming back needs a new invite. */
export function LeaveServer({ serverId, name }: { serverId: string; name: string }): ReactNode {
  const { t } = useTranslation();
  const [asking, setAsking] = useState(false);
  if (asking) {
    return (
      <ConfirmLeave
        serverId={serverId}
        name={name}
        cancel={() => {
          setAsking(false);
        }}
      />
    );
  }
  return (
    <Button
      label={t("servers.leave")}
      variant="danger"
      onPress={() => {
        setAsking(true);
      }}
      testID="server-leave"
    />
  );
}
