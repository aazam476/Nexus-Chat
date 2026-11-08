import { Button, Heading, Stack, Text } from "@nexus-chat/ui";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { useSmtpSettings } from "../../admin/useSmtpSettings";
import { ErrorMessage } from "../../components/ErrorMessage";
import { NoticeMessage } from "../../components/NoticeMessage";
import { SmtpForm } from "./SmtpForm";

/** The mail server the instance sends email through, and a test email to the admin. */
export function SmtpSection(): ReactNode {
  const { t } = useTranslation();
  const smtp = useSmtpSettings();
  return (
    <Stack>
      <Heading level={2}>{t("admin.smtp.title")}</Heading>
      <Text tone="muted">{t("admin.smtp.intro")}</Text>
      {smtp.settings === undefined ? (
        <Text tone="muted">{t("common.loading")}</Text>
      ) : (
        <SmtpForm
          saved={smtp.settings}
          busy={smtp.busy}
          fields={smtp.fields}
          onSave={(values, security) => void smtp.save(values, security)}
        />
      )}
      <Button
        label={t("admin.smtp.test")}
        variant="secondary"
        busy={smtp.busy}
        onPress={() => void smtp.test()}
        testID="smtp-test"
      />
      <ErrorMessage message={smtp.error} />
      <NoticeMessage message={smtp.notice} testID="smtp-notice" />
    </Stack>
  );
}
