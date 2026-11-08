import { Heading, Screen } from "@nexus-chat/ui";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { useRegister, type Registration } from "../src/auth/useRegister";
import { ErrorMessage } from "../src/components/ErrorMessage";
import { HttpsWarning } from "../src/components/HttpsWarning";
import { useConnection } from "../src/connection/ConnectionContext";
import { useSignedInRedirect } from "../src/navigation/useSignedInRedirect";
import { RegisterForm } from "../src/screens/register/RegisterForm";
import { useForm } from "../src/screens/useForm";

const empty: Registration = { username: "", displayName: "", email: "", password: "", inviteCode: "" };

/** Creating an account. */
export default function Register(): ReactNode {
  const { t } = useTranslation();
  const connection = useConnection();
  const flow = useRegister();
  const form = useForm(empty, flow.fields, "register");
  useSignedInRedirect();
  return (
    <Screen>
      <HttpsWarning />
      <Heading>{t("register.title")}</Heading>
      <ErrorMessage message={flow.error} />
      <RegisterForm
        form={form}
        askForInvite={connection.instance?.info.registrationMode === "invite_only"}
        busy={flow.busy}
        onSubmit={() => void flow.submit(form.values)}
      />
    </Screen>
  );
}
