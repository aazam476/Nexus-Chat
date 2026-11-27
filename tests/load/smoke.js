/**
 * The load smoke test (spec §7.2): every seeded user holds a gateway
 * connection, all in one server, while the server's owner sends messages
 * in its #general channel. Each listener times how long a message takes to
 * reach it, from the moment it was due to be sent to the moment the gateway
 * tells the listener about it. The test passes when the 95th percentile is
 * under 300 ms and (almost) every message reached every listener.
 *
 * Messages are sent on a fixed schedule from a start time every VU knows,
 * and each one's delivery ID is the first one's plus its number, so a
 * listener needs nothing from the sender to time it. A late send counts
 * against the server.
 *
 * Run: k6 run -e SEED=seed.json -e BASE_URL=http://localhost:8080 smoke.js
 * where seed.json comes from apps/server/test/load/seed.
 */
import { check, sleep } from "k6";
import http from "k6/http";
import { Counter, Trend } from "k6/metrics";
import { WebSocket } from "k6/experimental/websockets";
import { clearInterval, setInterval, setTimeout as later } from "k6/timers";

const seed = JSON.parse(open(__ENV.SEED || "seed.json"));
const base = __ENV.BASE_URL || "http://localhost:8080";
const users = seed.tokens.length;
const messages = Number(__ENV.MESSAGES || 30);
const intervalMs = Number(__ENV.INTERVAL_MS || 500);
// Time for every listener to connect and identify before the first message.
const connectMs = Number(__ENV.CONNECT_MS || 30000);

const latency = new Trend("delivery_latency", true);
const delivered = new Counter("messages_delivered");
const missent = new Counter("messages_missent");

export const options = {
  setupTimeout: "30s",
  scenarios: {
    listeners: {
      executor: "per-vu-iterations",
      exec: "listen",
      vus: users,
      iterations: 1,
      maxDuration: "5m",
    },
    sender: { executor: "per-vu-iterations", exec: "send", vus: 1, iterations: 1, maxDuration: "5m" },
  },
  thresholds: {
    delivery_latency: ["p(95)<300"],
    messages_delivered: [`count>=${Math.floor(users * messages * 0.99)}`],
    messages_missent: ["count==0"],
  },
};

function sendMessage(token) {
  return http.post(
    `${base}/api/v1/servers/${seed.serverId}/channels/${seed.channelId}/messages`,
    JSON.stringify({ message: seed.message }),
    { headers: { Authorization: `Bearer ${token}`, "Content-Type": "application/json" } },
  );
}

/** Sends one message to learn where delivery IDs stand, and fixes the schedule. */
export function setup() {
  const first = sendMessage(seed.tokens[0]);
  check(first, { "the first message is accepted": (response) => response.status === 201 });
  return { baseId: first.json("id"), start: Date.now() + connectMs };
}

/** When message `number` (1, 2, …) is due. */
function dueAt(data, number) {
  return data.start + (number - 1) * intervalMs;
}

/** One user's gateway connection: identify, keep it alive, and time each message. */
export function listen(data) {
  const token = seed.tokens[(__VU - 1) % users];
  const socket = new WebSocket(`${base.replace(/^http/, "ws")}/api/v1/gateway`, "nexus-chat.v1");
  let heartbeat;
  let lastSequence = 0;
  socket.onmessage = (event) => {
    const frame = JSON.parse(event.data);
    lastSequence = frame.seq || lastSequence;
    if (frame.type === "hello") {
      socket.send(JSON.stringify({ type: "identify", data: { token } }));
      heartbeat = setInterval(() => {
        socket.send(JSON.stringify({ type: "heartbeat", data: { lastSequence } }));
      }, frame.data.heartbeatIntervalMs);
    } else if (frame.type === "mls_messages" && frame.data.conversationId === seed.audienceId) {
      const number = frame.data.lastId - data.baseId;
      if (number >= 1 && number <= messages) {
        latency.add(Date.now() - dueAt(data, number));
        delivered.add(1);
      }
    }
  };
  later(
    () => {
      clearInterval(heartbeat);
      socket.close();
    },
    dueAt(data, messages) - Date.now() + 5000,
  );
}

/** The owner sends each message when it is due. */
export function send(data) {
  for (let number = 1; number <= messages; number++) {
    const wait = dueAt(data, number) - Date.now();
    if (wait > 0) {
      sleep(wait / 1000);
    }
    const response = sendMessage(seed.tokens[0]);
    if (response.status !== 201 || response.json("id") !== data.baseId + number) {
      missent.add(1);
    }
  }
}
