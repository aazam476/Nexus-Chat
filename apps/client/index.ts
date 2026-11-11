/**
 * The app's entry: the secure random source goes in before any route, and
 * so any crypto library, is loaded.
 */
import "./src/platform/secureRandom";
import "expo-router/entry";
