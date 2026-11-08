/**
 * Regression test for ts-mls issue #858: a ratchet tree whose parent node
 * lists a blank leaf among its unmerged leaves must be rejected (RFC 9420
 * §7.9.2: every unmerged leaf is a non-blank descendant). ts-mls 1.6.4
 * checked "blank and not a descendant" instead of "blank or not a
 * descendant"; patches/ts-mls\@1.6.4.patch carries the fix until upstream
 * ships it (ADR 0002).
 */
import { defaultAuthenticationService, type RatchetTree } from "ts-mls";
import { validateRatchetTree } from "ts-mls/clientState.js";
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
