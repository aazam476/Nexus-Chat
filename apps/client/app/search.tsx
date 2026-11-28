import { Heading, Link, Screen, Stack, Text } from "@nexus-chat/ui";
import { router, useLocalSearchParams } from "expo-router";
import { useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { HttpsWarning } from "../src/components/HttpsWarning";
import { useConnection } from "../src/connection/ConnectionContext";
import { SearchForm, type SearchRequest } from "../src/screens/search/SearchForm";
import { SearchResults } from "../src/screens/search/SearchResults";
import { usePlaces } from "../src/search/places";
import { filtersOf } from "../src/search/request";
import { useBackfill } from "../src/search/useBackfill";
import { useSearch } from "../src/search/useSearch";
import { OlderMessages } from "../src/screens/search/OlderMessages";

/**
 * Searching messages (spec §4.6). Messages are end-to-end encrypted, so
 * the search runs on this device, over the messages it has decrypted from
 * the last 90 days; the server never sees what is searched for.
 */
export default function SearchScreen(): ReactNode {
  const { t } = useTranslation();
  const { in: scope } = useLocalSearchParams<{ in?: string }>();
  const { instance } = useConnection();
  // Periods count back from when the screen opened.
  const [openedAt] = useState(() => new Date());
  const [request, setRequest] = useState<SearchRequest>({
    query: "",
    from: "",
    attachments: false,
    period: "any",
    here: scope !== undefined,
  });
  const filters = filtersOf(request, { scope, domain: instance?.info.domain ?? "", now: openedAt });
  const asked = request.query.trim() !== "" || Object.keys(filters).length > 0;
  const backfill = useBackfill(scope);
  const hits = useSearch(request.query, filters, backfill.state.kind);
  const { placeOf, nameOf } = usePlaces(
    (hits ?? []).map((hit) => hit.conversationId),
    t,
  );
  return (
    <Screen>
      <HttpsWarning />
      <Stack gap="sm">
        <Link
          label={t("search.back")}
          onPress={() => {
            router.back();
          }}
          testID="search-back"
        />
        <Heading>{t("search.title")}</Heading>
        <Text tone="muted">{t("search.intro")}</Text>
      </Stack>
      <SearchForm request={request} change={setRequest} scoped={scope !== undefined} />
      <SearchResults hits={asked ? hits : []} placeOf={placeOf} nameOf={nameOf} asked={asked} />
      {scope === undefined ? null : <OlderMessages state={backfill.state} start={backfill.start} />}
    </Screen>
  );
}
