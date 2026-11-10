/**
 * The database where no platform file applies (tests and type checking).
 * The real ones are `database.native.ts` and `database.web.ts`, chosen by
 * the bundler; the contract is in `localDatabase.ts`.
 */
import { createMemoryDatabase, type LocalDatabase } from "./localDatabase";

/** Opens this platform's database. */
export function openDatabase(): Promise<LocalDatabase> {
  return Promise.resolve(createMemoryDatabase());
}
