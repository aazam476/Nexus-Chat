import type { MfaMethod } from "@nexus-chat/api-client";
import { Link, Screen, Text } from "@nexus-chat/ui";
import { router } from "expo-router";
import { useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { useLogin } from "../src/auth/useLogin";
import { useProviderReturn } from "../src/auth/useProviderReturn";
import { ErrorMessage } from "../src/components/ErrorMessage";
import { useSignedInRedirect } from "../src/navigation/useSignedInRedirect";
import { SecondFactorForm } from "../src/screens/login/SecondFactorForm";

/** Where a provider sends the browser back to after sign-in (web). */
export default function ProviderReturn(): ReactNode {
  const { t } = useTranslation();
  const [fragment] = useState(() => (typeof window === "undefined" ? "" : window.location.hash));
  const state = useProviderReturn(fragment);
  useSignedInRedirect();
  return (
    <Screen>
      {state.kind === "working" && <Text>{t("oidc.finishing")}</Text>}
      {state.kind === "failed" && <ErrorMessage message={state.message} />}
      {state.kind === "secondFactor" && (
        <ProviderSecondFactor challenge={state.challenge.challengeToken} methods={state.challenge.methods} />
      )}
      {state.kind === "failed" && (
        <Link
          label={t("common.back")}
          onPress={() => {
            router.replace("/login");
          }}
        />
      )}
    </Screen>
  );
}

function ProviderSecondFactor({
  challenge,
  methods,
}: {
  challenge: string;
  methods: MfaMethod[];
}): ReactNode {
  const flow = useLogin({ kind: "secondFactor", challenge, methods });
  return (
    <>
      <ErrorMessage message={flow.error} />
      <SecondFactorForm
        methods={methods}
        busy={flow.busy}
        onSubmit={(method, code) => void flow.submitCode(method, code)}
        onCancel={() => {
          router.replace("/login");
        }}
      />
    </>
  );
}
