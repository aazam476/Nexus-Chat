/**
 * The client side of docs/realtime-protocol.md: identify, heartbeats,
 * resuming after a dropped connection with backoff, and handing events to
 * the app. It has no React in it, so it is easy to test with a fake socket.
 */

/** The subprotocol of protocol version 1. */
export const subprotocol = "nexus-chat.v1";

/** A server event: a frame with a sequence number. */
export interface GatewayEvent {
  type: string;
  seq: number;
  data: unknown;
}

/** The parts of WebSocket the client uses, so tests can pass a fake. */
export interface SocketLike {
  send(data: string): void;
  close(code?: number): void;
  onmessage: ((message: { data: unknown }) => void) | null;
  onclose: ((event: { code: number }) => void) | null;
}

/** Options of {@link GatewayClient}. */
export interface GatewayOptions {
  /** The instance's base address, http(s)://… */
  address: string;
  token: string;
  onEvent: (event: GatewayEvent) => void;
  /** The token was refused (close code 4004): the user must log in again. */
  onSignedOut: () => void;
  openSocket?: (url: string) => SocketLike;
  /** Random number in [0, 1), for jitter; replaced in tests. */
  random?: () => number;
}

const signedOutCode = 4004;
const stopCodes = new Set([signedOutCode, 4000, 4005]);
const maximumBackoffMs = 60_000;

interface Frame {
  type?: unknown;
  seq?: unknown;
  data?: unknown;
}

/** One device's connection to the gateway, reconnecting until stopped. */
export class GatewayClient {
  private socket: SocketLike | undefined;
  private sessionId: string | undefined;
  private lastSequence = 0;
  private heartbeat: ReturnType<typeof setInterval> | undefined;
  private retry: ReturnType<typeof setTimeout> | undefined;
  private failures = 0;
  private stopped = false;

  constructor(private readonly options: GatewayOptions) {}

  /** Opens the connection. */
  start(): void {
    this.stopped = false;
    this.open();
  }

  /** Closes the connection for good, for example on logout. */
  stop(): void {
    this.stopped = true;
    this.clearTimers();
    this.socket?.close(1000);
    this.socket = undefined;
  }

  /** Tells the server whether the user has been inactive for 10 minutes. */
  reportActivity(idle: boolean): void {
    this.send({ type: "activity", data: { idle } });
  }

  private open(): void {
    const url = `${this.options.address.replace(/^http/, "ws")}/api/v1/gateway`;
    const socket = (this.options.openSocket ?? openWebSocket)(url);
    this.socket = socket;
    socket.onmessage = (message) => {
      this.receive(message.data);
    };
    socket.onclose = (event) => {
      this.closed(event.code);
    };
  }

  private receive(raw: unknown): void {
    let frame: Frame;
    try {
      frame = JSON.parse(String(raw)) as Frame;
    } catch {
      return; // the server only sends JSON; ignore anything else
    }
    if (typeof frame.seq === "number") {
      this.lastSequence = frame.seq;
    }
    switch (frame.type) {
      case "hello":
        this.hello(frame.data as { heartbeatIntervalMs: number });
        break;
      case "ready":
        this.failures = 0;
        this.sessionId = (frame.data as { gatewaySessionId: string }).gatewaySessionId;
        this.deliver(frame);
        break;
      case "resumed":
        this.failures = 0;
        break;
      case "invalid_session":
        this.sessionId = undefined;
        this.lastSequence = 0;
        this.send({ type: "identify", data: { token: this.options.token } });
        break;
      default:
        this.deliver(frame);
    }
  }

  private hello(data: { heartbeatIntervalMs: number }): void {
    if (this.sessionId === undefined) {
      this.send({ type: "identify", data: { token: this.options.token } });
    } else {
      this.send({
        type: "resume",
        data: {
          token: this.options.token,
          gatewaySessionId: this.sessionId,
          lastSequence: this.lastSequence,
        },
      });
    }
    this.clearTimers();
    this.heartbeat = setInterval(() => {
      this.send({ type: "heartbeat", data: { lastSequence: this.lastSequence } });
    }, data.heartbeatIntervalMs);
  }

  private deliver(frame: Frame): void {
    if (typeof frame.type === "string" && typeof frame.seq === "number") {
      this.options.onEvent({ type: frame.type, seq: frame.seq, data: frame.data });
    }
  }

  private closed(code: number): void {
    this.clearTimers();
    this.socket = undefined;
    if (this.stopped) {
      return;
    }
    if (code === signedOutCode) {
      this.options.onSignedOut();
    }
    if (stopCodes.has(code)) {
      return;
    }
    // Exponential backoff with jitter (protocol §3): 1 s, 2 s, 4 s … 60 s.
    const base = Math.min(maximumBackoffMs, 1000 * 2 ** this.failures);
    this.failures++;
    const delay = base / 2 + (this.options.random ?? Math.random)() * (base / 2);
    this.retry = setTimeout(() => {
      this.open();
    }, delay);
  }

  private send(frame: { type: string; data: unknown }): void {
    this.socket?.send(JSON.stringify(frame));
  }

  private clearTimers(): void {
    clearInterval(this.heartbeat);
    clearTimeout(this.retry);
  }
}

function openWebSocket(url: string): SocketLike {
  return new WebSocket(url, subprotocol) as unknown as SocketLike;
}
