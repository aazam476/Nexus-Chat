import { Button, Heading, Screen, Stack, Text, TextField } from "@nexus-chat/ui";
import { router } from "expo-router";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { ErrorMessage } from "../src/components/ErrorMessage";
import { useConnectForm } from "../src/screens/useConnectForm";

/** Picks the instance to use (spec §3.6: one app, any instance). */
export default function Connect(): ReactNode {
  const { t } = useTranslation();
  const form = useConnectForm();
  const submit = async (): Promise<void> => {
    if (await form.submit()) {
      router.replace("/");
    }
  };
  return (
    <Screen>
      <Stack gap="sm">
        <Heading>{t("connect.title")}</Heading>
        <Text tone="muted">{t("connect.intro")}</Text>
      </Stack>
      <TextField
        label={t("connect.address")}
        hint={t("connect.addressHint")}
        value={form.address}
        onChangeText={form.setAddress}
        keyboardType="url"
        autoComplete="url"
        onSubmitEditing={() => void submit()}
        testID="instance-address"
      />
      <ErrorMessage message={form.error} />
      <Button label={t("common.continue")} busy={form.busy} onPress={() => void submit()} testID="connect" />
    </Screen>
  );
}
