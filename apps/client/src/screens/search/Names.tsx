import { Link, Text } from "@nexus-chat/ui";
import { useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { longEnough, useNameSearch } from "../../search/useNameSearch";
import { NameResults } from "./NameResults";

/**
 * Finding servers, channels and people by name. The words go to the
 * server only once the user asks, so a message search stays on the device.
 */
export function Names({ query }: { query: string }): ReactNode {
  const { t } = useTranslation();
  const [asked, setAsked] = useState(false);
  const found = useNameSearch(asked ? query : "");
  if (!asked) {
    return (
      <Link
        label={t("search.names.start")}
        onPress={() => {
          setAsked(true);
        }}
        testID="search-names-start"
      />
    );
  }
  if (!longEnough(query)) {
    return <Text tone="muted">{t("search.names.hint")}</Text>;
  }
  return found === undefined ? null : <NameResults found={found} />;
}
