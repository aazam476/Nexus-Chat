import type { RegistrationMode } from "@nexus-chat/api-client";
import { Button, Screen } from "@nexus-chat/ui";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { useSetup, type SetupForm } from "../src/auth/useSetup";
import { ErrorMessage } from "../src/components/ErrorMessage";
import { useConnection } from "../src/connection/ConnectionContext";
import { hostOf } from "../src/instance/address";
import { AdminFields } from "../src/screens/setup/AdminFields";
import { InstanceFields } from "../src/screens/setup/InstanceFields";
import { SetupDone } from "../src/screens/setup/SetupDone";
import { SetupIntro } from "../src/screens/setup/SetupIntro";
import { useForm } from "../src/screens/useForm";

const empty: SetupForm = {
  token: "",
  username: "",
  displayName: "",
  email: "",
  password: "",
  instanceName: "",
  domain: "",
  registrationMode: "invite_only",
};

/** The first-run wizard (spec §3.5). */
export default function Setup(): ReactNode {
  const { t } = useTranslation();
  const connection = useConnection();
  const flow = useSetup();
  const form = useForm(empty, flow.fields, "setup");
  const host = hostOf(connection.address ?? "http://localhost");
  if (flow.done) {
    return <SetupDone host={host} />;
  }
  const instance = connection.instance;
  return (
    <Screen>
      <SetupIntro host={host} secure={instance?.info.secureConnection !== false} />
      <ErrorMessage message={flow.error} />
      <AdminFields form={form} />
      <InstanceFields
        name={form.bind("instanceName", "instance.name")}
        domain={form.bind("domain", "instance.domain")}
        fixedDomain={instance?.domainFromEnvironment === true ? instance.info.domain : undefined}
        mode={form.values.registrationMode}
        onModeChange={(mode: RegistrationMode) => {
          form.setValue("registrationMode", mode);
        }}
      />
      <Button
        label={t("setup.submit")}
        busy={flow.busy}
        onPress={() => void flow.submit(form.values)}
        testID="setup-submit"
      />
    </Screen>
  );
}
