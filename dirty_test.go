package tetra3d

import "testing"

func wantWorldX(t *testing.T, name string, node INode, want float32) {
	t.Helper()
	if got := node.WorldPosition().X; got != want {
		t.Errorf("%s: world X is %v, want %v", name, got, want)
	}
}

// chain returns three nodes, each the child of the one before it.
func chain() (*Node, *Node, *Node) {
	p, c, g := NewNode("parent"), NewNode("child"), NewNode("grandchild")
	p.AddChildren(c)
	c.AddChildren(g)
	return p, c, g
}

func TestDirtyAfterParentRead(t *testing.T) {
	p, c, g := chain()
	c.SetLocalX(10)
	g.SetLocalX(100)
	p.SetLocalX(1)
	wantWorldX(t, "parent", p, 1)
	p.SetLocalX(2)
	wantWorldX(t, "child", c, 12)
	wantWorldX(t, "grandchild", g, 112)
}

func TestDirtyBelowDirtyChild(t *testing.T) {
	p, c, g := chain()
	wantWorldX(t, "grandchild", g, 0)
	c.SetLocalX(10)
	p.SetLocalX(1)
	wantWorldX(t, "grandchild", g, 11)
	p.SetLocalX(2)
	c.SetLocalX(20)
	p.SetLocalX(3)
	wantWorldX(t, "grandchild", g, 23)
}

func TestDirtyReparentCleanChild(t *testing.T) {
	p, _, _ := chain()
	other, leaf := NewNode("other"), NewNode("leaf")
	other.AddChildren(leaf)
	leaf.SetLocalX(5)
	wantWorldX(t, "leaf", leaf, 5)
	p.SetLocalX(1)
	p.AddChildren(leaf)
	wantWorldX(t, "leaf under dirty parent", leaf, 6)
	leaf.Unparent()
	wantWorldX(t, "leaf after unparent", leaf, 5)
	p.SetLocalX(2)
	p.AddChildren(leaf)
	wantWorldX(t, "parent", p, 2)
	p.SetLocalX(3)
	wantWorldX(t, "leaf after second move", leaf, 8)
}

func TestDirtyClone(t *testing.T) {
	p, c, g := chain()
	c.SetLocalX(10)
	wantWorldX(t, "grandchild", g, 10)
	clone := p.Clone().(*Node)
	cg := clone.Get("child/grandchild")
	wantWorldX(t, "clone root", clone, 0)
	clone.SetLocalX(1)
	wantWorldX(t, "clone grandchild", cg, 11)
	clone.SetLocalX(2)
	wantWorldX(t, "clone grandchild after second move", cg, 12)
	wantWorldX(t, "original grandchild", g, 10)
}

func TestDirtyReparentClearsSector(t *testing.T) {
	p, c, g := chain()
	c.SetLocalX(1)
	sector := &Sector{}
	g.cachedSector = sector
	c.Unparent()
	if g.cachedSector != nil {
		t.Error("unparent kept the Sector of a dirty grandchild")
	}
	g.cachedSector = sector
	NewNode("other").AddChildren(c)
	if g.cachedSector != nil {
		t.Error("reparent kept the Sector of a dirty grandchild")
	}
	p.AddChildren(c)
	g.cachedSector = sector
	clone := p.Clone().(*Node)
	if clone.Get("child/grandchild").(*Node).cachedSector != nil {
		t.Error("clone kept the Sector of a dirty grandchild")
	}
}
