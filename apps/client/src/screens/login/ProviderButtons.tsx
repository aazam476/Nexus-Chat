import type { SignInProvider } from "@nexus-chat/api-client";
import { Button, Stack, Text } from "@nexus-chat/ui";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

/** Props of {@link ProviderButtons}. */
export interface ProviderButtonsProps {
  providers: SignInProvider[];
  onSignIn: (providerId: string) => void;
}

/** One button per sign-in provider the instance offers; nothing if there are none. */
export function ProviderButtons({ providers, onSignIn }: ProviderButtonsProps): ReactNode {
  const { t } = useTranslation();
  if (providers.length === 0) {
    return null;
  }
  return (
    <Stack gap="sm">
      <Text tone="muted">{t("login.orProvider")}</Text>
      {providers.map((provider) => (
        <Button
          key={provider.id}
          variant="secondary"
          label={t("login.provider", { name: provider.displayName })}
          onPress={() => {
            onSignIn(provider.id);
          }}
          testID={`provider-${provider.id}`}
        />
      ))}
    </Stack>
  );
}
