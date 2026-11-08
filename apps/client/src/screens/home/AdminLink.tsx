import { Link } from "@nexus-chat/ui";
import { router } from "expo-router";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

/** The way into the Admin panel, shown only to instance admins. */
export function AdminLink({ admin }: { admin: boolean }): ReactNode {
  const { t } = useTranslation();
  if (!admin) {
    return null;
  }
  return (
    <Link
      label={t("home.admin")}
      onPress={() => {
        router.push("/admin");
      }}
      testID="go-admin"
    />
  );
}
