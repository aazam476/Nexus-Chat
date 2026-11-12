/**
 * Data every screen is checked against: kind demo data, and the worst
 * realistic data (code standards §8.5). Each worst-case value is either
 * at the limit the server enforces or a real-world shape that breaks
 * layouts: long compound names, unbreakable addresses, one-letter names,
 * right-to-left and non-Latin text, emoji-only names, counts of exactly 1.
 */
import type {
  CurrentUser,
  FieldError,
  InstanceInfo,
  PresenceStatus,
  SignInProvider,
  SmtpSettings,
  TlsStatus,
} from "@nexus-chat/api-client";

/** Everything a screen can show, as the API would return it. */
export interface Dataset {
  /** A short name for test titles and snapshot files. */
  name: string;
  instance: InstanceInfo;
  user: CurrentUser;
  /** The presence the gateway reports for the user's own devices. */
  presence: PresenceStatus;
  providers: SignInProvider[];
  tls: TlsStatus;
  smtp: SmtpSettings;
  /** Validation errors the sign-up and setup forms get back. */
  fieldErrors: FieldError[];
}

/** The time the checks run at, so dates and "expires in" counts never change. */
export const fixedNow = "2026-11-08T12:00:00Z";

/** Kind data: what the screens were designed with. */
export const demo: Dataset = {
  name: "demo",
  instance: {
    name: "Example Chat",
    domain: "chat.example.com",
    registrationMode: "open",
    secureConnection: true,
    signingKey: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
    limits: { groupDmMaxMembers: 25, attachmentMaxBytes: 26214400 },
  },
  user: {
    id: "0195f0e0-0000-7000-8000-000000000001",
    username: "ada",
    displayName: "Ada",
    email: "ada@example.com",
    emailVerified: true,
    isInstanceAdmin: true,
    bio: "",
    status: "online",
  },
  presence: "online",
  providers: [{ id: "company", displayName: "Company" }],
  tls: { active: true, names: ["chat.example.com"], issuer: "Example CA", expiresAt: "2027-02-06T12:00:00Z" },
  smtp: {
    configured: true,
    host: "smtp.example.com",
    port: 587,
    security: "starttls",
    username: "chat",
    from: "chat@example.com",
    passwordSet: true,
  },
  fieldErrors: [{ field: "username", message: "That username is taken." }],
};

const longDomain = "chat.northwind-industries-holdings.example.com";
const regions = ["eu-west-1", "eu-central-1", "us-east-1", "us-west-2", "ap-southeast-2", "sa-east-1"];
const services = ["media", "voice", "turn", "federation", "push", "files", "status"];

/**
 * The worst realistic data. Limits: instance name 64 characters, display
 * name and username 32, custom status 128, email 254, SMTP host 253; the
 * certificate's names and issuer are unbounded, so they are as long as
 * real corporate certificates get.
 */
export const worstCase: Dataset = {
  name: "worst-case",
  instance: {
    // 64 characters, the maximum, in Cyrillic with typographic quotes.
    name: "Северо-Западный клуб настольных игр «Ладья» — встречи по средам!",
    domain: longDomain,
    registrationMode: "open",
    // Plain HTTP, so every screen also carries the HTTPS warning.
    secureConnection: false,
    signingKey: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
    limits: { groupDmMaxMembers: 25, attachmentMaxBytes: 26214400 },
  },
  user: {
    id: "0195f0e0-0000-7000-8000-000000000002",
    username: "aleksandra.wisniewska_kowalczyk1",
    displayName: "Aleksandra Wiśniewska-Kowalczyk",
    email: "bartholomew.fitzgerald@northwind-industries-holdings.example.com",
    emailVerified: true,
    isInstanceAdmin: true,
    bio: "",
    status: "dnd",
    customStatus: {
      text: "Back on Thursday 🌴 — at the Wrocław offsite, reachable on Signal for anything urgent; otherwise ask Bartholomew in #support :)",
      expiresAt: "2026-11-12T09:00:00Z",
    },
  },
  presence: "dnd",
  providers: [
    { id: "northwind", displayName: "Northwind Industries Holdings Single Sign-On (Azure AD)" },
    { id: "g", displayName: "G" },
    { id: "arabic", displayName: "تسجيل الدخول الموحد لجامعة الملك عبدالعزيز" },
    { id: "emoji", displayName: "🔐🔐🔐" },
  ],
  tls: {
    active: true,
    names: [
      longDomain,
      `*.${longDomain}`,
      ...regions.map((region) => `${region}.${longDomain}`),
      ...services.map((service) => `${service}.${longDomain}`),
    ],
    issuer:
      "CN=Northwind Industries Holdings Issuing CA 2026 G2, OU=Information Security and Public Key Infrastructure, O=Northwind Industries Holdings Ltd, L=Manchester, C=GB",
    // One day left: the urgent warning, with a count of exactly 1.
    expiresAt: "2026-11-09T18:00:00Z",
  },
  smtp: {
    configured: true,
    host: "smtp-relay.outbound-mail-gateway.eu-central-1.northwind-industries-holdings.example.com",
    port: 465,
    security: "tls",
    username: "bartholomew.fitzgerald@northwind-industries-holdings.example.com",
    from: "no-reply.chat-notifications@northwind-industries-holdings.example.com",
    passwordSet: true,
  },
  fieldErrors: [
    {
      field: "username",
      message:
        "Usernames are 3–32 characters long and use only lowercase letters a–z, digits 0–9, underscores and dots; this one has a space.",
    },
    {
      field: "email",
      message: "An account already uses this email address. Log in to it, or reset its password.",
    },
    {
      field: "password",
      message:
        "This password appears in lists of breached passwords, so attackers try it first. Choose another one.",
    },
  ],
};

/** Right-to-left text in every name, and the shortest names allowed. */
export const rightToLeft: Dataset = {
  ...worstCase,
  name: "right-to-left",
  instance: { ...worstCase.instance, name: "نادي الشطرنج في جدة", secureConnection: true },
  user: {
    ...worstCase.user,
    username: "jo1",
    displayName: "عبد الرحمن بن عبد العزيز آل سعود",
    customStatus: { text: "🎉" },
    status: "online",
  },
  presence: "online",
  providers: [{ id: "x", displayName: "X" }],
};
