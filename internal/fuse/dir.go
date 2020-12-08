// +build darwin freebsd linux

package fuse

import (
	"context"
	"os"
	"path/filepath"
	"sync"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"

	"github.com/rubiojr/rapi/internal/debug"
	"github.com/rubiojr/rapi/restic"
)

// Statically ensure that *dir implement those interface
var _ = fs.HandleReadDirAller(&dir{})
var _ = fs.NodeStringLookuper(&dir{})

type dir struct {
	root        *Root
	items       map[string]*restic.Node
	inode       uint64
	parentInode uint64
	node        *restic.Node
	m           sync.Mutex
}

func cleanupNodeName(name string) string {
	return filepath.Base(name)
}

func newDir(ctx context.Context, root *Root, inode, parentInode uint64, node *restic.Node) (*dir, error) {
	debug.Log("new dir for %v (%v)", node.Name, node.Subtree)

	return &dir{
		root:        root,
		node:        node,
		inode:       inode,
		parentInode: parentInode,
	}, nil
}

// replaceSpecialNodes replaces nodes with name "." and "/" by their contents.
// Otherwise, the node is returned.
func replaceSpecialNodes(ctx context.Context, repo restic.Repository, node *restic.Node) ([]*restic.Node, error) {
	if node.Type != "dir" || node.Subtree == nil {
		return []*restic.Node{node}, nil
	}

	if node.Name != "." && node.Name != "/" {
		return []*restic.Node{node}, nil
	}

	tree, err := repo.LoadTree(ctx, *node.Subtree)
	if err != nil {
		return nil, err
	}

	return tree.Nodes, nil
}

func newDirFromSnapshot(ctx context.Context, root *Root, inode uint64, snapshot *restic.Snapshot) (*dir, error) {
	debug.Log("new dir for snapshot %v (%v)", snapshot.ID(), snapshot.Tree)
	return &dir{
		root: root,
		node: &restic.Node{
			AccessTime: snapshot.Time,
			ModTime:    snapshot.Time,
			ChangeTime: snapshot.Time,
			Mode:       os.ModeDir | 0555,
			Subtree:    snapshot.Tree,
		},
		inode: inode,
	}, nil
}

func (d *dir) open(ctx context.Context) error {
	d.m.Lock()
	defer d.m.Unlock()

	if d.items != nil {
		return nil
	}

	debug.Log("open dir %v (%v)", d.node.Name, d.node.Subtree)

	tree, err := d.root.repo.LoadTree(ctx, *d.node.Subtree)
	if err != nil {
		debug.Log("  error loading tree %v: %v", d.node.Subtree, err)
		return err
	}
	items := make(map[string]*restic.Node)
	for _, n := range tree.Nodes {
		nodes, err := replaceSpecialNodes(ctx, d.root.repo, n)
		if err != nil {
			debug.Log("  replaceSpecialNodes(%v) failed: %v", n, err)
			return err
		}
		for _, node := range nodes {
			items[cleanupNodeName(node.Name)] = node
		}
	}
	d.items = items
	return nil
}

func (d *dir) Attr(ctx context.Context, a *fuse.Attr) error {
	debug.Log("Attr()")
	a.Inode = d.inode
	a.Mode = os.ModeDir | d.node.Mode

	if !d.root.cfg.OwnerIsRoot {
		a.Uid = d.node.UID
		a.Gid = d.node.GID
	}
	a.Atime = d.node.AccessTime
	a.Ctime = d.node.ChangeTime
	a.Mtime = d.node.ModTime

	a.Nlink = d.calcNumberOfLinks()

	return nil
}

func (d *dir) calcNumberOfLinks() uint32 {
	// a directory d has 2 hardlinks + the number
	// of directories contained by d
	count := uint32(2)
	for _, node := range d.items {
		if node.Type == "dir" {
			count++
		}
	}
	return count
}

func (d *dir) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	debug.Log("ReadDirAll()")
	err := d.open(ctx)
	if err != nil {
		return nil, err
	}
	ret := make([]fuse.Dirent, 0, len(d.items)+2)

	ret = append(ret, fuse.Dirent{
		Inode: d.inode,
		Name:  ".",
		Type:  fuse.DT_Dir,
	})

	ret = append(ret, fuse.Dirent{
		Inode: d.parentInode,
		Name:  "..",
		Type:  fuse.DT_Dir,
	})

	for _, node := range d.items {
		name := cleanupNodeName(node.Name)
		var typ fuse.DirentType
		switch node.Type {
		case "dir":
			typ = fuse.DT_Dir
		case "file":
			typ = fuse.DT_File
		case "symlink":
			typ = fuse.DT_Link
		}

		ret = append(ret, fuse.Dirent{
			Inode: fs.GenerateDynamicInode(d.inode, name),
			Type:  typ,
			Name:  name,
		})
	}

	return ret, nil
}

func (d *dir) Lookup(ctx context.Context, name string) (fs.Node, error) {
	debug.Log("Lookup(%v)", name)

	err := d.open(ctx)
	if err != nil {
		return nil, err
	}

	node, ok := d.items[name]
	if !ok {
		debug.Log("  Lookup(%v) -> not found", name)
		return nil, fuse.ENOENT
	}
	switch node.Type {
	case "dir":
		return newDir(ctx, d.root, fs.GenerateDynamicInode(d.inode, name), d.inode, node)
	case "file":
		return newFile(ctx, d.root, fs.GenerateDynamicInode(d.inode, name), node)
	case "symlink":
		return newLink(ctx, d.root, fs.GenerateDynamicInode(d.inode, name), node)
	case "dev", "chardev", "fifo", "socket":
		return newOther(ctx, d.root, fs.GenerateDynamicInode(d.inode, name), node)
	default:
		debug.Log("  node %v has unknown type %v", name, node.Type)
		return nil, fuse.ENOENT
	}
}

func (d *dir) Listxattr(ctx context.Context, req *fuse.ListxattrRequest, resp *fuse.ListxattrResponse) error {
	debug.Log("Listxattr(%v, %v)", d.node.Name, req.Size)
	for _, attr := range d.node.ExtendedAttributes {
		resp.Append(attr.Name)
	}
	return nil
}

func (d *dir) Getxattr(ctx context.Context, req *fuse.GetxattrRequest, resp *fuse.GetxattrResponse) error {
	debug.Log("Getxattr(%v, %v, %v)", d.node.Name, req.Name, req.Size)
	attrval := d.node.GetExtendedAttribute(req.Name)
	if attrval != nil {
		resp.Xattr = attrval
		return nil
	}
	return fuse.ErrNoXattr
}
