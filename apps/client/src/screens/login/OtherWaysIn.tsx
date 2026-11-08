import { Button, Link, Stack } from "@nexus-chat/ui";
import { router } from "expo-router";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { usePasskeyLogin } from "../../auth/usePasskeyLogin";
import type { Providers } from "../../auth/useProviders";
import { useConnection } from "../../connection/ConnectionContext";
import { defaultAddress } from "../../instance/defaultAddress";
import { ProviderButtons } from "./ProviderButtons";

/** Props of {@link OtherWaysIn}. */
export interface OtherWaysInProps {
  passkey: ReturnType<typeof usePasskeyLogin>;
  providers: Providers;
}

/** Passkey, providers, sign-up and switching instance. */
export function OtherWaysIn({ passkey, providers }: OtherWaysInProps): ReactNode {
  const { t } = useTranslation();
  const connection = useConnection();
  const registrationOpen = connection.instance?.info.registrationMode !== "closed";
  return (
    <Stack>
      {passkey.supported && (
        <Button
          variant="secondary"
          label={t("login.passkey")}
          busy={passkey.busy}
          onPress={() => void passkey.logIn()}
          testID="passkey-login"
        />
      )}
      <ProviderButtons providers={providers.providers} onSignIn={(id) => void providers.signIn(id)} />
      {registrationOpen && (
        <Link
          label={t("login.register")}
          onPress={() => {
            router.push("/register");
          }}
          testID="go-register"
        />
      )}
      {/* A web build served by an instance belongs to it; the apps and the
          desktop app can switch. */}
      {defaultAddress() === undefined && (
        <Link
          label={t("login.otherInstance")}
          onPress={() =>
            void connection.forgetInstance().then(() => {
              router.replace("/connect");
            })
          }
        />
      )}
    </Stack>
  );
}
