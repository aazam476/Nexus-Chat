import { Button, Heading, Stack, Text, TextField } from "@nexus-chat/ui";
import { useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { isolate } from "../../i18n/bidi";

/** Props of {@link AddFriend}. */
export interface AddFriendProps {
  request: (username: string) => Promise<boolean>;
}

/** The username being typed and the last request's result. */
function useAddFriend(request: AddFriendProps["request"]): {
  username: string;
  setUsername: (text: string) => void;
  result: { sent: boolean; to: string } | undefined;
  busy: boolean;
  submit: () => void;
} {
  const [username, setUsername] = useState("");
  const [result, setResult] = useState<{ sent: boolean; to: string }>();
  const [busy, setBusy] = useState(false);
  const submit = (): void => {
    const to = username.trim().replace(/^@/u, "");
    if (to === "" || busy) {
      return;
    }
    setBusy(true);
    void request(to).then((sent) => {
      setBusy(false);
      setResult({ sent, to });
      if (sent) {
        setUsername("");
      }
    });
  };
  return { username, setUsername, result, busy, submit };
}

/** Sends a friend request by username. */
export function AddFriend({ request }: AddFriendProps): ReactNode {
  const { t } = useTranslation();
  const { username, setUsername, result, busy, submit } = useAddFriend(request);
  return (
    <Stack gap="sm">
      <Heading level={2}>{t("friends.add")}</Heading>
      <TextField
        label={t("friends.username")}
        hint={t("friends.usernameHint")}
        value={username}
        onChangeText={setUsername}
        autoCapitalize="none"
        autoComplete="off"
        onSubmitEditing={submit}
        testID="friend-username"
      />
      {result === undefined ? null : (
        <Text tone={result.sent ? "muted" : "danger"} live>
          {result.sent ? t("friends.sent", { name: isolate(result.to) }) : t("friends.sendFailed")}
        </Text>
      )}
      <Button
        label={t("friends.send")}
        variant="secondary"
        onPress={submit}
        busy={busy}
        testID="friend-send"
      />
    </Stack>
  );
}
