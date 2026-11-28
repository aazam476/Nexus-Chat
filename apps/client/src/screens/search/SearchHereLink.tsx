import { Link } from "@nexus-chat/ui";
import { router } from "expo-router";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

/** Opens search, limited to the conversation or channel `id` until the user widens it. */
export function SearchHereLink({ id }: { id: string }): ReactNode {
  const { t } = useTranslation();
  return (
    <Link
      label={t("search.openHere")}
      onPress={() => {
        router.push({ pathname: "/search", params: { in: id } });
      }}
      testID="search-here"
    />
  );
}
