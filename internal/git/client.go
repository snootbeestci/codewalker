package git

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// Client wraps a go-git repository for safe concurrent reads.
type Client struct {
	repo *gogit.Repository
	root string
}

// Open opens the git repository at repoPath.
func Open(repoPath string) (*Client, error) {
	repo, err := gogit.PlainOpen(repoPath)
	if err != nil {
		return nil, fmt.Errorf("git: open %s: %w", repoPath, err)
	}
	return &Client{repo: repo, root: repoPath}, nil
}

// Root returns the repository root path.
func (c *Client) Root() string { return c.root }

// ReadFile returns the contents of filePath at the given git ref.
// ref may be a branch name, tag, or full commit SHA.
// If ref is empty, the working-tree file is read directly.
func (c *Client) ReadFile(ref, filePath string) ([]byte, error) {
	if ref == "" {
		return c.readWorkingTree(filePath)
	}

	commit, err := c.resolveRef(ref)
	if err != nil {
		return nil, err
	}

	tree, err := commit.Tree()
	if err != nil {
		return nil, fmt.Errorf("git: get tree at %s: %w", ref, err)
	}

	f, err := tree.File(filePath)
	if err != nil {
		return nil, fmt.Errorf("git: file %q at %s: %w", filePath, ref, err)
	}

	r, err := f.Reader()
	if err != nil {
		return nil, fmt.Errorf("git: open %q: %w", filePath, err)
	}
	defer r.Close()

	return io.ReadAll(r)
}

// ListFiles returns all file paths in the repository tree at ref.
// If ref is empty, uses HEAD.
func (c *Client) ListFiles(ref string) ([]string, error) {
	if ref == "" {
		ref = "HEAD"
	}
	commit, err := c.resolveRef(ref)
	if err != nil {
		return nil, err
	}
	tree, err := commit.Tree()
	if err != nil {
		return nil, fmt.Errorf("git: get tree: %w", err)
	}

	var paths []string
	err = tree.Files().ForEach(func(f *object.File) error {
		paths = append(paths, f.Name)
		return nil
	})
	return paths, err
}

// resolveRef resolves a ref string to a Commit object.
func (c *Client) resolveRef(ref string) (*object.Commit, error) {
	// Try as a hash first.
	if len(ref) >= 7 {
		h := plumbing.NewHash(ref)
		commit, err := c.repo.CommitObject(h)
		if err == nil {
			return commit, nil
		}
	}

	// Try as a symbolic ref (branch, tag).
	hash, err := c.repo.ResolveRevision(plumbing.Revision(ref))
	if err != nil {
		// Fall back to refs/heads/ and refs/tags/ prefixes.
		for _, prefix := range []string{"refs/heads/", "refs/tags/"} {
			hash, err = c.repo.ResolveRevision(plumbing.Revision(prefix + ref))
			if err == nil {
				break
			}
		}
		if err != nil {
			return nil, fmt.Errorf("git: resolve %q: %w", ref, err)
		}
	}

	commit, err := c.repo.CommitObject(*hash)
	if err != nil {
		return nil, fmt.Errorf("git: commit %s: %w", hash, err)
	}
	return commit, nil
}

// readWorkingTree reads a file directly from disk (no git involvement).
func (c *Client) readWorkingTree(filePath string) ([]byte, error) {
	return os.ReadFile(filepath.Join(c.root, filePath))
}
