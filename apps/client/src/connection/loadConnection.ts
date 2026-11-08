/**
 * Reading an instance's public state and the signed-in user, without any
 * React. The connection hook calls these; tests call them directly.
 */
import {
  getCurrentUser,
  getInstanceInfo,
  getSetupStatus,
  type Client,
  type CurrentUser,
  type InstanceInfo,
} from "@nexus-chat/api-client";

/** What the app needs to know about an instance before anyone logs in. */
export interface InstanceState {
  info: InstanceInfo;
  setupRequired: boolean;
  /** The instance's domain is fixed by DOMAIN in its .env. */
  domainFromEnvironment: boolean;
}

/** The instance's public state, or undefined when it does not answer as a Nexus Chat instance. */
export async function loadInstance(client: Client): Promise<InstanceState | undefined> {
  try {
    const [info, setup] = await Promise.all([getInstanceInfo({ client }), getSetupStatus({ client })]);
    if (info.data === undefined || setup.data === undefined) {
      return undefined;
    }
    return {
      info: info.data,
      setupRequired: setup.data.setupRequired,
      domainFromEnvironment: setup.data.domainFromEnvironment,
    };
  } catch {
    return undefined; // not reachable, or not JSON
  }
}

/** The signed-in user, or undefined when the token no longer works. */
export async function loadUser(client: Client): Promise<CurrentUser | undefined> {
  const result = await getCurrentUser({ client });
  return result.data;
}
