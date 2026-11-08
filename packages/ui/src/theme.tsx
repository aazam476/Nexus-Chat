/**
 * The current theme. The user picks light, dark, or follow the system
 * (spec §4.8); every component reads its colors through `useTheme`.
 */
import { createContext, use, useMemo, type ReactNode } from "react";
import { useColorScheme } from "react-native";

import { colors, type Colors } from "./tokens";

/** What the user chose in settings. */
export type ThemePreference = "system" | "light" | "dark";

/** The theme in effect. */
export interface Theme {
  scheme: "light" | "dark";
  colors: Colors;
}

const ThemeContext = createContext<Theme>({ scheme: "light", colors: colors.light });

/** Props of {@link ThemeProvider}. */
export interface ThemeProviderProps {
  preference: ThemePreference;
  children: ReactNode;
}

/** Provides the theme for `preference` to everything inside it. */
export function ThemeProvider({ preference, children }: ThemeProviderProps): ReactNode {
  const system = useColorScheme();
  const scheme = preference === "system" ? (system === "dark" ? "dark" : "light") : preference;
  const theme = useMemo(() => ({ scheme, colors: colors[scheme] }), [scheme]);
  return <ThemeContext value={theme}>{children}</ThemeContext>;
}

/** The theme in effect. */
export function useTheme(): Theme {
  return use(ThemeContext);
}
