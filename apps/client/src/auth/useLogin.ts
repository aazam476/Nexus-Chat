/**
 * The password login flow (spec §4.1): username or email and password,
 * then a second factor when the account has one. A wrong second factor
 * ends the attempt, as the server allows one try per challenge.
 */
import type { MfaMethod } from "@nexus-chat/api-client";
import { useState } from "react";

import { useConnection } from "../connection/ConnectionContext";
import type { Message } from "./errors";
import { passwordStep, secondFactorStep, type StepOutcome } from "./loginSteps";

/** Where the login is. */
export type LoginStep =
  { kind: "password" } | { kind: "secondFactor"; challenge: string; methods: MfaMethod[] };

/** State and actions of the login screen. */
export interface LoginFlow {
  step: LoginStep;
  busy: boolean;
  error: Message | undefined;
  submitPassword: (loginName: string, password: string) => Promise<void>;
  submitCode: (method: MfaMethod, code: string) => Promise<void>;
  startOver: () => void;
}

/**
 * Runs the login flow against the current instance. A provider sign-in
 * that needs a second factor starts at `initialStep`.
 */
export function useLogin(initialStep: LoginStep = { kind: "password" }): LoginFlow {
  const connection = useConnection();
  const [step, setStep] = useState<LoginStep>(initialStep);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<Message | undefined>(undefined);
  const run = async (attempt: () => Promise<StepOutcome>): Promise<void> => {
    setBusy(true);
    setError(undefined);
    const outcome = await attempt();
    setBusy(false);
    if (outcome.kind === "signedIn") {
      await connection.signedIn(outcome.result.token, outcome.result.user);
    } else if (outcome.kind === "secondFactor") {
      setStep(outcome);
    } else {
      setError(outcome.message);
      setStep({ kind: "password" });
    }
  };
  const submitCode = (method: MfaMethod, code: string): Promise<void> =>
    step.kind === "secondFactor"
      ? run(() => secondFactorStep(connection.client, step.challenge, method, code))
      : Promise.resolve();
  return {
    step,
    busy,
    error,
    submitPassword: (loginName, password) => run(() => passwordStep(connection.client, loginName, password)),
    submitCode,
    startOver: () => {
      setStep({ kind: "password" });
    },
  };
}
