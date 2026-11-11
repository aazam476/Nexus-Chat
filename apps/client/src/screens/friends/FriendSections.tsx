import type { Contact, Friends } from "@nexus-chat/api-client";
import { Heading, Stack, Text } from "@nexus-chat/ui";
import { router } from "expo-router";
import { useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { isolate } from "../../i18n/bidi";
import { ContactRow, type ContactRowProps } from "./ContactRow";
import type { FriendsState } from "./useFriends";

interface SectionProps {
  title: string;
  contacts: Contact[];
  actionsFor: (contact: Contact) => ContactRowProps["actions"];
}

function Section({ title, contacts, actionsFor }: SectionProps): ReactNode {
  if (contacts.length === 0) {
    return null;
  }
  return (
    <Stack gap="xs">
      <Heading level={2}>{title}</Heading>
      {contacts.map((contact) => (
        <ContactRow key={contact.user.id} contact={contact} actions={actionsFor(contact)} />
      ))}
    </Stack>
  );
}

/** Requests for the user and requests they sent. */
function Requests({ friends, state }: { friends: Friends; state: FriendsState }): ReactNode {
  const { t } = useTranslation();
  return (
    <>
      <Section
        title={t("friends.incoming")}
        contacts={friends.incoming}
        actionsFor={({ user }) => [
          {
            label: t("friends.accept"),
            onPress: () => void state.accept(user.id),
            testID: `accept-${user.username}`,
          },
          { label: t("friends.decline"), onPress: () => void state.withdraw(user.id) },
        ]}
      />
      <Section
        title={t("friends.outgoing")}
        contacts={friends.outgoing}
        actionsFor={({ user }) => [
          { label: t("friends.cancel"), onPress: () => void state.withdraw(user.id) },
        ]}
      />
    </>
  );
}

/** The user's friends, each with a way to message them. */
function FriendList({ friends, state }: { friends: Friends; state: FriendsState }): ReactNode {
  const { t } = useTranslation();
  const [failed, setFailed] = useState<string>();
  const message = (contact: Contact) => () => {
    setFailed(undefined);
    void state.message(contact.user.id).then((id) => {
      if (id === undefined) {
        setFailed(t("friends.messageFailed", { name: isolate(contact.user.displayName) }));
      } else {
        router.push({ pathname: "/conversation/[id]", params: { id } });
      }
    });
  };
  return (
    <>
      {failed === undefined ? null : (
        <Text tone="danger" live>
          {failed}
        </Text>
      )}
      {friends.friends.length === 0 ? <Text tone="muted">{t("friends.none")}</Text> : null}
      <Section
        title={t("friends.friends")}
        contacts={friends.friends}
        actionsFor={(contact) => [
          {
            label: t("friends.message"),
            onPress: message(contact),
            testID: `message-${contact.user.username}`,
          },
          { label: t("friends.remove"), onPress: () => void state.remove(contact.user.id) },
        ]}
      />
    </>
  );
}

/** Requests both ways, and the user's friends. */
export function FriendSections({ state }: { state: FriendsState }): ReactNode {
  const { friends } = state;
  if (friends === undefined) {
    return null;
  }
  return (
    <Stack gap="lg">
      <Requests friends={friends} state={state} />
      <FriendList friends={friends} state={state} />
    </Stack>
  );
}
