/**
 * Changing a group DM from its settings screen: each action runs one at a
 * time and reports a message the screen shows. The conversation reloads
 * through conversation_update, which every change sends.
 */
import { router } from "expo-router";
import { useState } from "react";
import { useTranslation } from "react-i18next";

import { useConnection } from "../../connection/ConnectionContext";
import {
  addToGroup,
  leaveGroup,
  removeGroupIcon,
  renameGroup,
  setGroupIcon,
  type GroupResult,
} from "../../messaging/groupActions";
import { pickSquareImage } from "../../platform/pickImage";
import { groupErrorText } from "./errorText";

/** What the settings screen can do, and how the last action went. */
export interface GroupSettings {
  busy: boolean;
  error: string | undefined;
  rename: (name: string) => void;
  chooseIcon: () => void;
  removeIcon: () => void;
  add: (userId: string) => void;
  leave: () => void;
}

type Client = NonNullable<ReturnType<typeof useConnection>["client"]>;
type Action = (client: Client) => Promise<GroupResult | undefined>;

/** Runs one group action at a time and keeps its error message. */
function useRunner(): {
  busy: boolean;
  error: string | undefined;
  run: (action: Action, then?: () => void) => void;
} {
  const { t } = useTranslation();
  const { client, instance } = useConnection();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();
  const maximum = instance?.info.limits.groupDmMaxMembers ?? 25;
  const run = (action: Action, then?: () => void): void => {
    if (client === undefined || busy) {
      return;
    }
    setBusy(true);
    setError(undefined);
    void action(client).then((result) => {
      setBusy(false);
      if (result?.ok === false) {
        setError(groupErrorText(result.code, t, maximum));
      } else if (result !== undefined) {
        then?.();
      }
    });
  };
  return { busy, error, run };
}

/** Actions on the group DM `conversationId`. */
export function useGroupSettings(conversationId: string): GroupSettings {
  const { busy, error, run } = useRunner();
  return {
    busy,
    error,
    rename: (name) => {
      run((client) => renameGroup(client, conversationId, name));
    },
    chooseIcon: () => {
      run(async (client) => {
        const image = await pickSquareImage();
        return image === undefined ? undefined : setGroupIcon(client, conversationId, image);
      });
    },
    removeIcon: () => {
      run((client) => removeGroupIcon(client, conversationId));
    },
    add: (userId) => {
      run((client) => addToGroup(client, conversationId, userId));
    },
    leave: () => {
      run(
        (client) => leaveGroup(client, conversationId),
        () => {
          router.replace("/conversations");
        },
      );
    },
  };
}
