/**
 * The MLS ciphersuite every Nexus Chat group uses: 0x0001,
 * MLS_128_DHKEMX25519_AES128GCM_SHA256_Ed25519, the suite every RFC 9420
 * implementation must support (RFC 9420 §17.1; docs/e2ee-design.md). It
 * is fixed per group; a later move to a post-quantum suite creates new
 * groups.
 */
import { getCiphersuiteFromName, getCiphersuiteImpl, type CiphersuiteImpl } from "ts-mls";

/** The suite's registered name. */
export const ciphersuiteName = "MLS_128_DHKEMX25519_AES128GCM_SHA256_Ed25519";

let loaded: Promise<CiphersuiteImpl> | undefined;

/** The suite's primitives, loaded once and shared. */
export function ciphersuite(): Promise<CiphersuiteImpl> {
  loaded ??= getCiphersuiteImpl(getCiphersuiteFromName(ciphersuiteName));
  return loaded;
}
