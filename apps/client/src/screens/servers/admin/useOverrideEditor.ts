/** One channel override for its editor: each permission's state, and saving it. */
import type { OverrideTargetKind, PermissionName } from "@nexus-chat/api-client";
import { useState } from "react";

import { saveOverride } from "../../../servers/manageActions";
import { specOf, statesOf, type OverrideState } from "../../../servers/overrides";
import type { LoadedServer } from "../../../servers/useServers";
import type { ServerRunner } from "../useServerRunner";

/** Where the override is: the server, the channel, and who it is for. */
export interface OverrideWhere {
  server: LoadedServer;
  channelId: string;
  targetKind: OverrideTargetKind;
  targetId: string;
}

/** The editor's state and its save. */
export interface OverrideEditor {
  states: ReadonlyMap<PermissionName, OverrideState>;
  set: (permission: PermissionName, state: OverrideState) => void;
  save: (then: () => void) => void;
}

/** Starts from the override as stored, if there is one. */
export function useOverrideEditor(where: OverrideWhere, runner: ServerRunner): OverrideEditor {
  const { server, channelId, targetKind, targetId } = where;
  const current = server.details.channels
    .find((channel) => channel.id === channelId)
    ?.overrides.find((override) => override.targetKind === targetKind && override.targetId === targetId);
  const [states, setStates] = useState(() => statesOf(current));
  return {
    states,
    set: (permission, state) => {
      setStates((previous) => new Map(previous).set(permission, state));
    },
    save: (then) => {
      const target = { serverId: server.details.server.id, channelId, targetKind, targetId };
      runner.run((client) => saveOverride(client, target, specOf(states)), then);
    },
  };
}
