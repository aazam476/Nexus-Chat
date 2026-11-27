import { useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { emojiToken } from "../../servers/emojiTokens";
import { useOfferedEmoji } from "../../servers/ServerEmoji";
import { ActionButton } from "./ActionButton";
import { CustomEmoji } from "./CustomEmoji";

/** How many of the server's emoji are offered before "More emoji". */
const firstFew = 4;

/**
 * Reacting with the server's custom emoji: the first few by name, and the
 * rest behind "More emoji", so a server with a hundred does not bury the
 * message's other actions.
 */
export function CustomReactions({ react }: { react: (emoji: string, added: boolean) => void }): ReactNode {
  const { t } = useTranslation();
  const offered = useOfferedEmoji();
  const [all, setAll] = useState(false);
  const shown = all ? offered : offered.slice(0, firstFew);
  return (
    <>
      {shown.map((emoji) => (
        <ActionButton
          key={emoji.id}
          icon={<CustomEmoji name={emoji.name} id={emoji.id} />}
          label=""
          accessibilityLabel={t("conversation.react", { emoji: `:${emoji.name}:` })}
          onPress={() => {
            react(emojiToken(emoji), true);
          }}
          testID={`react-${emoji.name}`}
        />
      ))}
      {offered.length > firstFew ? (
        <ActionButton
          label={all ? t("conversation.fewerEmoji") : t("conversation.moreEmoji")}
          onPress={() => {
            setAll(!all);
          }}
          testID="more-emoji"
        />
      ) : null}
    </>
  );
}
