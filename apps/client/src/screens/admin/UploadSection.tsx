import { Button, Heading, Stack, Text, TextField } from "@nexus-chat/ui";
import { useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { useUploadSettings } from "../../admin/useUploadSettings";
import { ErrorMessage } from "../../components/ErrorMessage";
import { NoticeMessage } from "../../components/NoticeMessage";
import { fileSize } from "../../messaging/fileSize";

/** The largest attachment people can send on this instance. */
export function UploadSection(): ReactNode {
  const { t } = useTranslation();
  const uploads = useUploadSettings();
  const [megabytes, setMegabytes] = useState("");
  return (
    <Stack>
      <Heading level={2}>{t("admin.uploads.title")}</Heading>
      <Text tone="muted">{t("admin.uploads.intro")}</Text>
      {uploads.limit === undefined ? (
        <Text tone="muted">{t("common.loading")}</Text>
      ) : (
        <Text testID="upload-limit">{t("admin.uploads.current", { limit: fileSize(uploads.limit, t) })}</Text>
      )}
      <TextField
        label={t("admin.uploads.field")}
        hint={t("admin.uploads.fieldHint")}
        value={megabytes}
        onChangeText={setMegabytes}
        keyboardType="number-pad"
        testID="upload-limit-field"
      />
      <Button
        label={t("admin.uploads.save")}
        variant="secondary"
        busy={uploads.busy}
        onPress={() => void uploads.save(megabytes)}
        testID="upload-limit-save"
      />
      <ErrorMessage message={uploads.error} />
      <NoticeMessage message={uploads.notice} testID="upload-notice" />
    </Stack>
  );
}
