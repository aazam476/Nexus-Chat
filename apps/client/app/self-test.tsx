import { Heading, Screen, Stack, Text } from "@nexus-chat/ui";
import { Redirect } from "expo-router";
import { useEffect, useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { runVectors, type VectorProgress } from "../src/selfTest/vectorRun";

/** Set only in CI's end-to-end Android build, with the vector server's address. */
const vectorsAddress = (): string | undefined => process.env.EXPO_PUBLIC_NEXUS_VECTORS_URL;

function Results({
  progress,
  error,
}: {
  progress: VectorProgress | undefined;
  error: string | undefined;
}): ReactNode {
  const { t } = useTranslation();
  if (error !== undefined) {
    return (
      <Text tone="danger" testID="self-test-failed">
        {error}
      </Text>
    );
  }
  if (progress === undefined) {
    return <Text tone="muted">{t("selfTest.loading")}</Text>;
  }
  const finished = progress.done === progress.total;
  return (
    <Stack gap="sm">
      <Text testID={finished && progress.failures.length === 0 ? "self-test-passed" : "self-test-progress"}>
        {t("selfTest.progress", {
          done: progress.done,
          total: progress.total,
          failed: progress.failures.length,
        })}
      </Text>
      {progress.failures.map((failure) => (
        <Text key={failure} tone="danger" testID={finished ? "self-test-failed" : undefined}>
          {failure}
        </Text>
      ))}
    </Stack>
  );
}

/** Runs the MLS interoperability vectors on this device; CI test builds only. */
export default function SelfTest(): ReactNode {
  const { t } = useTranslation();
  const [progress, setProgress] = useState<VectorProgress>();
  const [error, setError] = useState<string>();
  const vectorsUrl = vectorsAddress();
  useEffect(() => {
    if (vectorsUrl === undefined) {
      return;
    }
    runVectors(vectorsUrl, setProgress).catch((cause: unknown) => {
      setError(String(cause));
    });
  }, [vectorsUrl]);
  if (vectorsUrl === undefined) {
    return <Redirect href="/" />;
  }
  return (
    <Screen>
      <Stack gap="md">
        <Heading>{t("selfTest.title")}</Heading>
        <Results progress={progress} error={error} />
      </Stack>
    </Screen>
  );
}
