import type { Contact } from "@nexus-chat/api-client";
import { Avatar, ListRow, spacing } from "@nexus-chat/ui";
import type { ReactNode } from "react";
import { StyleSheet, View } from "react-native";

import { useConnection } from "../../connection/ConnectionContext";
import { isolate } from "../../i18n/bidi";
import { ActionButton } from "../conversation/ActionButton";

/** Props of {@link ContactRow}. */
export interface ContactRowProps {
  contact: Contact;
  actions: { label: string; onPress: () => void; testID?: string }[];
}

/** A person in a friends list, with the actions that apply. */
export function ContactRow({ contact, actions }: ContactRowProps): ReactNode {
  const { address } = useConnection();
  const { user } = contact;
  const name = isolate(user.displayName);
  return (
    <View style={styles.row}>
      <ListRow
        title={name}
        subtitle={`@${user.username}`}
        leading={
          <Avatar
            name={user.displayName}
            {...(user.avatarUrl === undefined || address === undefined
              ? {}
              : { imageUrl: `${address}${user.avatarUrl}` })}
          />
        }
      />
      <View style={styles.actions}>
        {actions.map((action) => (
          <ActionButton key={action.label} {...action} />
        ))}
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  row: { gap: spacing.xs },
  actions: { flexDirection: "row", flexWrap: "wrap", gap: spacing.sm, paddingHorizontal: spacing.lg },
});
