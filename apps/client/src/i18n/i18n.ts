/**
 * Translations (spec §4.13). Every string the user sees comes from
 * `locales/<language>.json`; only English exists for beta, and adding a
 * language means adding a file here.
 */
import i18next, { type i18n as I18n } from "i18next";
import { initReactI18next, useTranslation } from "react-i18next";

import en from "../../locales/en.json";
import type { Message } from "../auth/errors";

/** Creates the app's translation instance, ready to use at once. */
export function createI18n(): I18n {
  const instance = i18next.createInstance();
  void instance.use(initReactI18next).init({
    lng: "en",
    fallbackLng: "en",
    resources: { en: { translation: en } },
    // React already escapes text, so i18next must not do it twice.
    interpolation: { escapeValue: false },
    initAsync: false,
  });
  return instance;
}

/** The text for a message from a hook. */
export function useMessageText(message: Message | undefined): string | undefined {
  const { t } = useTranslation();
  return message === undefined ? undefined : t(message.key, message.values);
}
