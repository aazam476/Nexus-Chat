import { Heading, Screen } from "@nexus-chat/ui";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { usePasskeyLogin } from "../src/auth/usePasskeyLogin";
import { useLogin } from "../src/auth/useLogin";
import { useProviders } from "../src/auth/useProviders";
import { ErrorMessage } from "../src/components/ErrorMessage";
import { HttpsWarning } from "../src/components/HttpsWarning";
import { useConnection } from "../src/connection/ConnectionContext";
import { hostOf } from "../src/instance/address";
import { useSignedInRedirect } from "../src/navigation/useSignedInRedirect";
import { OtherWaysIn } from "../src/screens/login/OtherWaysIn";
import { PasswordForm } from "../src/screens/login/PasswordForm";
import { SecondFactorForm } from "../src/screens/login/SecondFactorForm";

/** Logging in: password (then a second factor), passkey, or a provider. */
export default function Login(): ReactNode {
  const { t } = useTranslation();
  const connection = useConnection();
  const flow = useLogin();
  const passkey = usePasskeyLogin();
  const providers = useProviders();
  useSignedInRedirect();
  const instanceName = connection.instance?.info.name ?? hostOf(connection.address ?? "http://localhost");
  return (
    <Screen>
      <HttpsWarning />
      <Heading>{t("login.title", { instance: instanceName })}</Heading>
      <ErrorMessage message={flow.error ?? passkey.error ?? providers.error} />
      {flow.step.kind === "secondFactor" ? (
        <SecondFactorForm
          methods={flow.step.methods}
          busy={flow.busy}
          onSubmit={(method, code) => void flow.submitCode(method, code)}
          onCancel={flow.startOver}
        />
      ) : (
        <>
          <PasswordForm
            busy={flow.busy}
            onSubmit={(login, password) => void flow.submitPassword(login, password)}
          />
          <OtherWaysIn passkey={passkey} providers={providers} />
        </>
      )}
    </Screen>
  );
}
