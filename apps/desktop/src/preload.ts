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
  /** Wraps the database key with the OS secret store; undefined where there is none. */
  protectKey: (key: Uint8Array): Promise<Uint8Array | undefined> =>
    ipcRenderer.invoke("protect-key", key) as Promise<Uint8Array | undefined>,
  /** Unwraps a key that protectKey wrapped. */
  unprotectKey: (sealed: Uint8Array): Promise<Uint8Array> =>
    ipcRenderer.invoke("unprotect-key", sealed) as Promise<Uint8Array>,
});
