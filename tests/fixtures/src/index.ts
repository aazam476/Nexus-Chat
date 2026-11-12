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
  UserSummary,
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
  /** The conversations screens: who the user talks to, and what they send. */
  messaging: {
    /** The first is the user's DM; all of them are in the group. */
    contacts: UserSummary[];
    groupName: string;
    /** Unread messages in the group, for its badge. */
    groupUnread: number;
    /** Sent in the DM, in order. */
    messages: string[];
  };
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
    linkPreviewProxy: false,
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
  messaging: {
    contacts: [
      { id: "0195f0e0-0000-7000-8000-000000000101", username: "grace", displayName: "Grace" },
      { id: "0195f0e0-0000-7000-8000-000000000102", username: "linus", displayName: "Linus" },
    ],
    groupName: "Book club",
    groupUnread: 2,
    messages: ["Are we still on for Thursday?", "I'll bring the **second** volume."],
  },
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
    linkPreviewProxy: false,
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
  messaging: {
    contacts: [
      {
        id: "0195f0e0-0000-7000-8000-000000000201",
        username: "maximilian.von_hohenstein_wittelsbach",
        displayName: "Maximilian von Hohenstein-Wittelsbach-Löwenstein",
      },
      { id: "0195f0e0-0000-7000-8000-000000000202", username: "q", displayName: "Q" },
      { id: "0195f0e0-0000-7000-8000-000000000203", username: "nadia", displayName: "نادية الحسيني" },
      { id: "0195f0e0-0000-7000-8000-000000000204", username: "emoji", displayName: "🦄🦄🦄" },
    ],
    // 64 characters, the maximum.
    groupName: "Planning committee for the 2027 Wrocław offsite & regional sync",
    groupUnread: 1,
    messages: [
      "Here is the report: https://northwind-industries-holdings.example.com/reports/quarterly/2026/q3/final-final-v2-reviewed.pdf",
      "Supercalifragilisticexpialidocious-antidisestablishmentarianism-pneumonoultramicroscopicsilicovolcanoconiosis",
      "> quoted from earlier\n\n**bold**, *italic*, `code`, ~~struck~~ and ||a spoiler|| 🎉",
      "1",
    ],
  },
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
  messaging: {
    ...worstCase.messaging,
    groupName: "مجموعة الشطرنج",
    messages: ["مرحبا، هل نلتقي يوم الخميس؟", "نعم، في الساعة ٧ مساءً 👍"],
  },
};
