import type { UserSummary } from "@nexus-chat/api-client";
import { Banner, Button, Stack } from "@nexus-chat/ui";
import { useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { isolate } from "../../i18n/bidi";
import type { Messenger } from "../../messaging/messenger";

/** Props of {@link IdentityWarning}. */
export interface IdentityWarningProps {
  messenger: Messenger;
  members: UserSummary[];
}

/**
 * Warns when a member's encryption identity changed since this device
 * pinned it (docs/e2ee-design.md §2); their devices are refused until the
 * user chooses to trust the new identity.
 */
export function IdentityWarning({ messenger, members }: IdentityWarningProps): ReactNode {
  const { t } = useTranslation();
  const [changed, setChanged] = useState(() => messenger.identityChanges());
  const affected = members.filter((member) => changed.includes(member.id));
  return affected.map((member) => (
    <Banner key={member.id} title={t("conversation.identityChanged", { name: isolate(member.displayName) })}>
      <Stack gap="sm">
        <Button
          label={t("conversation.acceptIdentity")}
          variant="secondary"
          onPress={() => {
            void messenger.acceptIdentityChange(member.id).then(() => {
              setChanged(messenger.identityChanges());
            });
          }}
        />
      </Stack>
    </Banner>
  ));
}
