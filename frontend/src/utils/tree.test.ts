import { describe, it, expect } from "vitest";
import { handleTree } from "./tree";

describe("handleTree", () => {
  it("把扁平数组按 parentId 组装成树", () => {
    const flat = [
      { id: 1, parentId: 0, name: "root" },
      { id: 2, parentId: 1, name: "child-a" },
      { id: 3, parentId: 1, name: "child-b" },
      { id: 4, parentId: 2, name: "grandchild" }
    ];
    const tree = handleTree(flat);

    expect(tree).toHaveLength(1);
    expect(tree[0].name).toBe("root");
    expect(tree[0].children).toHaveLength(2);
    // 深层嵌套
    const childA = tree[0].children.find((n: any) => n.id === 2);
    expect(childA.children).toHaveLength(1);
    expect(childA.children[0].name).toBe("grandchild");
  });

  it("非数组输入返回空数组", () => {
    expect(handleTree(null as any)).toEqual([]);
  });

  it("支持自定义 id/parentId 字段名", () => {
    const flat = [
      { uid: "a", pid: null },
      { uid: "b", pid: "a" }
    ];
    const tree = handleTree(flat, "uid", "pid");
    expect(tree).toHaveLength(1);
    expect(tree[0].children[0].uid).toBe("b");
  });
});
