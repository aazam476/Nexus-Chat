/**
 * The only bridge from the web build to the desktop shell. The page runs
 * sandboxed, without Node.js; it can ask for exactly this and nothing
 * else (Electron security checklist, items 2–4).
 */
import { contextBridge, ipcRenderer } from "electron";

contextBridge.exposeInMainWorld("nexusDesktop", {
  /** Opens the provider in its own window and resolves with the return address's fragment. */
  signInWithProvider: (authorizationUrl: string): Promise<string | undefined> =>
    ipcRenderer.invoke("provider-sign-in", authorizationUrl) as Promise<string | undefined>,
});
