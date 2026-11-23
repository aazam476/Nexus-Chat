import type { InvitePreview } from "@nexus-chat/api-client";
import { Button, Stack, Text, TextField } from "@nexus-chat/ui";
import { router } from "expo-router";
import { useCallback, useEffect, useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { useConnection } from "../../connection/ConnectionContext";
import { isolate } from "../../i18n/bidi";
import { acceptInvite, inviteCodeOf, lookUpInvite, serverErrorText } from "../../servers/serverActions";
import { ServerIcon } from "./ServerIcon";

/** What the invite leads to, and the button that joins. */
function Invitation({ preview, code }: { preview: InvitePreview; code: string }): ReactNode {
  const { t } = useTranslation();
  const { client } = useConnection();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();
  const name = isolate(preview.server.name);
  const join = (): void => {
    if (client === undefined) {
      return;
    }
    setBusy(true);
    void acceptInvite(client, code).then((result) => {
      setBusy(false);
      if (result.ok) {
        router.replace({ pathname: "/server/[id]", params: { id: result.id } });
      } else {
        setError(serverErrorText(result.code, t));
      }
    });
  };
  return (
    <Stack gap="sm">
      <ServerIcon server={preview.server} size={64} />
      <Text>{t("servers.invitedTo", { name })}</Text>
      {preview.server.description === "" ? null : (
        <Text tone="muted">{isolate(preview.server.description)}</Text>
      )}
      <Text tone="muted">{t("servers.members", { count: preview.members })}</Text>
      {error === undefined ? null : (
        <Text tone="danger" live>
          {error}
        </Text>
      )}
      <Button label={t("servers.joinButton", { name })} onPress={join} busy={busy} testID="invite-join" />
    </Stack>
  );
}

/** What was looked up: the code, and where it leads if it is valid. */
interface Looked {
  code: string;
  preview: InvitePreview | undefined;
}

/** Looks invites up; the code the screen opened with is looked up at once. */
function useInviteLookup(initialCode: string): {
  looked: Looked | undefined;
  lookUp: (typed: string) => void;
} {
  const { client } = useConnection();
  const [looked, setLooked] = useState<Looked>();
  const lookUp = useCallback(
    (typed: string): void => {
      if (client === undefined) {
        return;
      }
      const code = inviteCodeOf(typed);
      void lookUpInvite(client, code).then((preview) => {
        setLooked({ code, preview });
      });
    },
    [client],
  );
  useEffect(() => {
    if (initialCode !== "") {
      lookUp(initialCode);
    }
  }, [initialCode, lookUp]);
  return { looked, lookUp };
}

/** The result of a look-up: what the invite leads to, or that it is not valid. */
function LookUpResult({ looked }: { looked: Looked | undefined }): ReactNode {
  const { t } = useTranslation();
  if (looked === undefined) {
    return null;
  }
  return looked.preview === undefined ? (
    <Text tone="danger" live>
      {t("servers.errors.invite_invalid")}
    </Text>
  ) : (
    <Invitation preview={looked.preview} code={looked.code} />
  );
}

/**
 * Joining by invite: the code or link, what it leads to, and joining. An
 * invite link opens this with its code filled in.
 */
export function JoinServer({ initialCode = "" }: { initialCode?: string }): ReactNode {
  const { t } = useTranslation();
  const [text, setText] = useState(initialCode);
  const { looked, lookUp } = useInviteLookup(initialCode);
  return (
    <Stack gap="sm">
      <TextField
        label={t("servers.inviteField")}
        value={text}
        onChangeText={setText}
        autoCapitalize="none"
        onSubmitEditing={() => {
          lookUp(text);
        }}
        testID="invite-code"
      />
      <Button
        label={t("servers.lookUp")}
        variant="secondary"
        onPress={() => {
          lookUp(text);
        }}
        disabled={text.trim() === ""}
        testID="invite-look-up"
      />
      <LookUpResult looked={looked} />
    </Stack>
  );
}
