/**
 * Leaves the login, sign-up and provider pages once someone is signed in.
 */
import { router } from "expo-router";
import { useEffect } from "react";

import { useConnection } from "../connection/ConnectionContext";

/** Goes to the home screen as soon as a user is signed in. */
export function useSignedInRedirect(): void {
  const { user } = useConnection();
  useEffect(() => {
    if (user !== undefined) {
      router.replace("/home");
    }
  }, [user]);
}
