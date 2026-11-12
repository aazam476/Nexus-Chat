import { Button, Heading, Link, Screen, Stack, Text, TextField } from "@nexus-chat/ui";
import { router } from "expo-router";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { HttpsWarning } from "../src/components/HttpsWarning";
import { useFriends } from "../src/screens/friends/useFriends";
import { FriendChoices } from "../src/screens/groups/FriendChoices";
import { useNewGroup } from "../src/screens/groups/useNewGroup";

/** Why the last attempt failed, and the button that creates the group. */
function CreateGroup({ group }: { group: ReturnType<typeof useNewGroup> }): ReactNode {
  const { t } = useTranslation();
  return (
    <>
      {group.error === undefined ? null : (
        <Text tone="danger" live>
          {group.error}
        </Text>
      )}
      <Button
        label={t("groups.create")}
        onPress={group.create}
        busy={group.busy}
        disabled={group.chosen.size < 2}
        testID="group-create"
      />
    </>
  );
}

/** Starts a group DM with some of the user's friends. */
export default function NewGroup(): ReactNode {
  const { t } = useTranslation();
  const { friends } = useFriends();
  const group = useNewGroup();
  return (
    <Screen>
      <HttpsWarning />
      <Stack gap="sm">
        <Link
          label={t("groups.back")}
          onPress={() => {
            router.back();
          }}
          testID="new-group-back"
        />
        <Heading>{t("groups.newTitle")}</Heading>
        <Text tone="muted">{t("groups.intro")}</Text>
      </Stack>
      <TextField
        label={t("groups.name")}
        hint={t("groups.nameHint")}
        value={group.name}
        onChangeText={group.setName}
        testID="group-name"
      />
      <FriendChoices
        friends={friends?.friends ?? []}
        chosen={group.chosen}
        room={group.room}
        toggle={group.toggle}
      />
      <CreateGroup group={group} />
    </Screen>
  );
}
