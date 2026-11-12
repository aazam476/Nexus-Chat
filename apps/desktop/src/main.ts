/**
 * The desktop app (spec §2): a window around the bundled web build, served
 * from the `app://nexus-chat` origin so that the browser APIs the app needs
 * (WebCrypto, IndexedDB) treat it as a secure context. Native features
 * (screen picker, tray, push-to-talk) arrive in milestone M11.
 */
import { app, BrowserWindow, ipcMain, net, protocol, safeStorage, session, shell } from "electron";
import path from "node:path";
import { pathToFileURL } from "node:url";

import { appOrigin, contentTypeFor, fileFor } from "./files";
import { protectKey, unprotectKey } from "./keyProtection";
import { fetchPreviewResource } from "./previews";
import {
  contentSecurityPolicy,
  isAppAddress,
  isExternalLink,
  isWebPage,
  providerReturnFragment,
} from "./navigation";

protocol.registerSchemesAsPrivileged([
  { scheme: "app", privileges: { standard: true, secure: true, supportFetchAPI: true } },
]);

/** Where the web build is: next to the app when packaged, or the client's build in development. */
function webDirectory(): string {
  return (
    process.env.NEXUS_WEB_DIR ??
    (app.isPackaged ? path.join(process.resourcesPath, "web") : path.join(__dirname, "../../client/dist"))
  );
}

function serveWebBuild(): void {
  const directory = webDirectory();
  protocol.handle("app", async (request) => {
    const file = fileFor(request.url, directory);
    const response = await net.fetch(pathToFileURL(file).toString());
    return new Response(response.body, {
      headers: { "Content-Type": contentTypeFor(file), "Content-Security-Policy": contentSecurityPolicy },
    });
  });
}

function createWindow(): BrowserWindow {
  const window = new BrowserWindow({
    title: "Nexus Chat",
    width: 1100,
    height: 760,
    webPreferences: {
      preload: path.join(__dirname, "preload.js"),
      contextIsolation: true,
      sandbox: true,
      nodeIntegration: false,
    },
  });
  window.webContents.setWindowOpenHandler(({ url }) => {
    if (isExternalLink(url)) {
      void shell.openExternal(url);
    }
    return { action: "deny" };
  });
  window.webContents.on("will-navigate", (event, url) => {
    if (!isAppAddress(url)) {
      event.preventDefault();
    }
  });
  void window.loadURL(`${appOrigin}/`);
  return window;
}

/**
 * Provider sign-in (ADR 0016): the provider opens in its own window, and
 * when the instance redirects it to the return address, the window closes
 * and the fragment goes to the app, which finishes the sign-in.
 */
function handleProviderSignIn(): void {
  ipcMain.handle("provider-sign-in", (event, authorizationUrl: unknown) => {
    if (typeof authorizationUrl !== "string" || !isWebPage(authorizationUrl)) {
      return Promise.resolve(undefined);
    }
    const parent = BrowserWindow.fromWebContents(event.sender) ?? undefined;
    const window = new BrowserWindow({
      parent,
      modal: true,
      width: 520,
      height: 700,
      webPreferences: { sandbox: true, contextIsolation: true },
    });
    return new Promise<string | undefined>((resolve) => {
      const check = (navigation: Electron.Event, url: string): void => {
        const fragment = providerReturnFragment(url);
        if (fragment !== undefined) {
          navigation.preventDefault();
          resolve(fragment);
          window.close();
        }
      };
      window.webContents.on("will-redirect", check);
      window.webContents.on("will-navigate", check);
      window.on("closed", () => {
        resolve(undefined); // closed by the user; a resolved promise ignores a second answer
      });
      void window.loadURL(authorizationUrl);
    });
  });
}

/**
 * Database-key wrapping for the web build (keyProtection.ts). Only the
 * app's own pages may ask; a provider's window never can.
 */
function handleKeyProtection(): void {
  const fromApp = (event: Electron.IpcMainInvokeEvent): boolean => isAppAddress(event.senderFrame?.url ?? "");
  ipcMain.handle("protect-key", (event, key: unknown) => {
    if (!fromApp(event)) {
      throw new Error("only the app may protect keys");
    }
    return protectKey(safeStorage, key);
  });
  ipcMain.handle("unprotect-key", (event, sealed: unknown) => {
    if (!fromApp(event)) {
      throw new Error("only the app may unprotect keys");
    }
    return unprotectKey(safeStorage, sealed);
  });
}

/**
 * Link previews (previews.ts): the app's own pages may ask the shell to
 * fetch a page or image, which it does directly, as a browser would.
 */
function handlePreviews(): void {
  ipcMain.handle("fetch-preview", (event, url: unknown, kind: unknown) => {
    if (!isAppAddress(event.senderFrame?.url ?? "")) {
      throw new Error("only the app may fetch previews");
    }
    return fetchPreviewResource((address, init) => net.fetch(address, init), url, kind);
  });
}

void app.whenReady().then(() => {
  // No page may use the camera, microphone or notifications until the
  // features that need them ask the user (M8, M9).
  session.defaultSession.setPermissionRequestHandler((_contents, _permission, callback) => {
    callback(false);
  });
  serveWebBuild();
  handleProviderSignIn();
  handleKeyProtection();
  handlePreviews();
  createWindow();
  app.on("activate", () => {
    if (BrowserWindow.getAllWindows().length === 0) {
      createWindow();
    }
  });
});

app.on("window-all-closed", () => {
  if (process.platform !== "darwin") {
    app.quit();
  }
});
