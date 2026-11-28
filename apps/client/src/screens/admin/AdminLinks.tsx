import { Link, Stack } from "@nexus-chat/ui";
import { router } from "expo-router";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

const pages = [
  { label: "admin.sections.users", pathname: "/admin-users", testID: "admin-users" },
  { label: "admin.sections.servers", pathname: "/admin-servers", testID: "admin-servers" },
  { label: "admin.sections.reports", pathname: "/admin-reports", testID: "admin-reports" },
] as const;

/** The admin pages with lists of their own: accounts, servers and reports. */
export function AdminLinks(): ReactNode {
  const { t } = useTranslation();
  return (
    <Stack gap="sm">
      {pages.map(({ label, pathname, testID }) => (
        <Link
          key={pathname}
          label={t(label)}
          onPress={() => {
            router.push(pathname);
          }}
          testID={testID}
        />
      ))}
    </Stack>
  );
}
