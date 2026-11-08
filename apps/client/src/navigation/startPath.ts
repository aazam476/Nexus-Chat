/**
 * Which screen fits the connection's state: pick an instance, set it up,
 * log in, or carry on.
 */
import type { Connection } from "../connection/useConnectionState";

/** A screen to start on. */
export type StartPath = "/connect" | "/setup" | "/login" | "/home";

/** The screen for `connection`. */
export function startPath(connection: Pick<Connection, "address" | "instance" | "user">): StartPath {
  if (connection.address === undefined || connection.instance === undefined) {
    return "/connect";
  }
  if (connection.instance.setupRequired) {
    return "/setup";
  }
  return connection.user === undefined ? "/login" : "/home";
}
