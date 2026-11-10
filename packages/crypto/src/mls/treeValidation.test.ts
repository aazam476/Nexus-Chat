/**
 * Regression test for ts-mls issue #858: a ratchet tree whose parent node
 * lists a blank leaf among its unmerged leaves must be rejected (RFC 9420
 * §7.9.2: every unmerged leaf is a non-blank descendant). ts-mls 1.6.4
 * checked "blank and not a descendant" instead of "blank or not a
 * descendant"; patches/ts-mls\@1.6.4.patch carries the fix until upstream
 * ships it (ADR 0002). The same patch caches subtree hashes; the cache must
 * never hand back the hash of a node that has changed.
 */
import { defaultAuthenticationService, type RatchetTree } from "ts-mls";
import { validateRatchetTree } from "ts-mls/clientState.js";
import { treeHashRoot } from "ts-mls/treeHash.js";
import { defaultClientConfig } from "ts-mls/clientConfig.js";

import { ciphersuite } from "./ciphersuite.js";
import { addMembers, createGroup, createMember, updateKeys, type Group } from "./engine.js";

const text = (value: string): Uint8Array => new TextEncoder().encode(value);

async function threeMemberGroup(): Promise<Group> {
  const alice = await createMember(text("alice"));
  const bob = await createMember(text("bob"));
  const carol = await createMember(text("carol"));
  const group = await createGroup(text("g"), alice);
  const added = (await addMembers(group, [bob.keyPackage, carol.keyPackage])).group;
  // Adding alone sends no update path; this commit fills in the parent nodes.
  return (await updateKeys(added)).group;
}

async function validate(group: Group, tree: RatchetTree): Promise<string | undefined> {
  const { groupContext } = group.state;
  const error = await validateRatchetTree(
    tree,
    groupContext,
    defaultClientConfig.lifetimeConfig,
    defaultAuthenticationService,
    groupContext.treeHash,
    await ciphersuite(),
  );
  return error?.message;
}

describe("ratchet tree validation", () => {
  it("accepts the tree of a real group", async () => {
    const group = await threeMemberGroup();

    expect(await validate(group, group.state.ratchetTree)).toBeUndefined();
  });

  it("hashes a changed node again instead of reusing the cached subtree hash", async () => {
    const group = await threeMemberGroup();
    const { hash } = await ciphersuite();
    const original = group.state.ratchetTree;
    const before = await treeHashRoot(original, hash);
    // Same nodes everywhere except one leaf, replaced by a changed copy.
    const changed: RatchetTree = original.map((node, index) =>
      index === 2 && node?.nodeType === "leaf"
        ? { ...node, leaf: { ...node.leaf, signature: new Uint8Array(node.leaf.signature.length) } }
        : node,
    );
    // A new hash object starts with an empty cache: the reference result.
    const uncached = { ...hash, digest: (data: Uint8Array) => hash.digest(data) };

    const after = await treeHashRoot(changed, hash);
    expect(after).not.toEqual(before);
    expect(after).toEqual(await treeHashRoot(changed, uncached));
    expect(await treeHashRoot(original, hash)).toEqual(before);
  });

  it("rejects a parent that lists a blank leaf as unmerged", async () => {
    const group = await threeMemberGroup();
    // Leaves 0–2 are members; leaf 3 (node 6) is blank. The root (node 3)
    // now claims leaf 3 as an unmerged descendant.
    const tree: RatchetTree = group.state.ratchetTree.map((node, index) =>
      index === 3 && node?.nodeType === "parent"
        ? { ...node, parent: { ...node.parent, unmergedLeaves: [3] } }
        : node,
    );
    expect(tree[3]?.nodeType).toBe("parent");
    expect(tree[6]).toBeUndefined();

    expect(await validate(group, tree)).toBe(
      "Unmerged leaf did not represent a non-blank descendant leaf node",
    );
  });
});
