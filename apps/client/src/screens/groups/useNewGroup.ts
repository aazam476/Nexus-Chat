/** The choices on the New group screen, and creating the group. */
import { router } from "expo-router";
import { useState } from "react";
import { useTranslation } from "react-i18next";

import { useConnection } from "../../connection/ConnectionContext";
import { createGroup } from "../../messaging/groupActions";
import { groupErrorText } from "./errorText";

/** The people chosen, the name, and how to create the group. */
export interface NewGroup {
  chosen: ReadonlySet<string>;
  toggle: (userId: string) => void;
  name: string;
  setName: (name: string) => void;
  /** Others the group may have besides the user. */
  room: number;
  /** Creates the group and opens it, or explains why it could not. */
  create: () => void;
  busy: boolean;
  error: string | undefined;
}

/** `chosen` with `userId` taken out, or added if there is room. */
function toggled(chosen: ReadonlySet<string>, userId: string, room: number): ReadonlySet<string> {
  const next = new Set(chosen);
  if (!next.delete(userId) && next.size < room) {
    next.add(userId);
  }
  return next;
}

/** Keeps the New group screen's state. */
export function useNewGroup(): NewGroup {
  const { t } = useTranslation();
  const { client, instance } = useConnection();
  const [chosen, setChosen] = useState<ReadonlySet<string>>(() => new Set());
  const [name, setName] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();
  const maximum = instance?.info.limits.groupDmMaxMembers ?? 25;
  const room = maximum - 1;
  const create = (): void => {
    if (client === undefined) {
      return;
    }
    setBusy(true);
    void createGroup(client, [...chosen], name).then((result) => {
      setBusy(false);
      if (result.ok) {
        router.replace({ pathname: "/conversation/[id]", params: { id: result.id } });
      } else {
        setError(groupErrorText(result.code, t, maximum));
      }
    });
  };
  return {
    chosen,
    toggle: (userId) => {
      setChosen((current) => toggled(current, userId, room));
    },
    name,
    setName,
    room,
    create,
    busy,
    error,
  };
}
