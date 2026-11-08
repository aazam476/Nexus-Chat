import type { RegistrationMode } from "@nexus-chat/api-client";
import { Heading, Text, TextField } from "@nexus-chat/ui";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

import type { FieldBinding } from "../useForm";
import { RegistrationModePicker } from "./RegistrationModePicker";

/** Props of {@link InstanceFields}. */
export interface InstanceFieldsProps {
  name: FieldBinding;
  domain: FieldBinding;
  /** The domain set by DOMAIN in .env, which the wizard cannot change. */
  fixedDomain: string | undefined;
  mode: RegistrationMode;
  onModeChange: (mode: RegistrationMode) => void;
}

/** The instance's name, domain and registration mode. */
export function InstanceFields({
  name,
  domain,
  fixedDomain,
  mode,
  onModeChange,
}: InstanceFieldsProps): ReactNode {
  const { t } = useTranslation();
  return (
    <>
      <Heading level={2}>{t("setup.instanceSection")}</Heading>
      <TextField label={t("setup.instanceName")} autoCapitalize="words" {...name} />
      {fixedDomain === undefined ? (
        <TextField label={t("setup.domain")} hint={t("setup.domainHint")} keyboardType="url" {...domain} />
      ) : (
        <Text tone="muted">{t("setup.domainFromEnvironment", { domain: fixedDomain })}</Text>
      )}
      <RegistrationModePicker value={mode} onChange={onModeChange} />
    </>
  );
}
