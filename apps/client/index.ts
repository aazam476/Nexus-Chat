/**
 * The app's entry: WebCrypto goes in before any route, and so any crypto
 * library, is loaded.
 */
import "./src/platform/webCrypto";
import "expo-router/entry";
