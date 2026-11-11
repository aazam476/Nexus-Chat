/**
 * A gateway socket that tests drive, and a way to have the app open it
 * instead of a real WebSocket.
 */
import type { SocketLike } from "../realtime/gatewayClient";

/** A socket the test drives: it records what the client sends. */
export class FakeSocket implements SocketLike {
  sent: { type: string; data: Record<string, unknown> }[] = [];
  closedWith: number | undefined;
  onmessage: ((message: { data: unknown }) => void) | null = null;
  onclose: ((event: { code: number }) => void) | null = null;
  constructor(readonly url: string) {}
  send(data: string): void {
    this.sent.push(JSON.parse(data) as { type: string; data: Record<string, unknown> });
  }
  close(code?: number): void {
    this.closedWith = code;
  }
  receive(frame: unknown): void {
    this.onmessage?.({ data: typeof frame === "string" ? frame : JSON.stringify(frame) });
  }
  drop(code: number): void {
    this.onclose?.({ code });
  }
}

/** The socket at `index`; fails the test clearly when there is none. */
export function nth(sockets: FakeSocket[], index: number): FakeSocket {
  const socket = sockets[index];
  if (socket === undefined) {
    throw new Error(`no socket ${String(index)} was opened`);
  }
  return socket;
}

/** Makes every WebSocket the app opens a FakeSocket, collected in the returned list. */
export function fakeSockets(): FakeSocket[] {
  const sockets: FakeSocket[] = [];
  (globalThis as { WebSocket?: unknown }).WebSocket = jest.fn((url: string) => {
    const socket = new FakeSocket(url);
    sockets.push(socket);
    return socket;
  });
  return sockets;
}
