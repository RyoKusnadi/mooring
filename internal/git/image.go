package git

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/ryokusnadi/mooring/internal/files"
)

// Image is one side of the diff of an image file, as Diff and CommitDiff compare it:
// without commit, the staged version against HEAD (staged) or the working file against the
// staged version; with commit, the commit's version against its first parent. old picks the
// earlier side. A side the file does not have (added, deleted) is NOT_FOUND.
func (s *Service) Image(ctx context.Context, projectID, folder, rel, commit string, staged, old bool) ([]byte, string, error) {
	pl, err := s.need(projectID, folder)
	if err != nil {
		return nil, "", err
	}
	spec, name := "", rel
	if commit != "" {
		c, hit, err := s.commitFile(ctx, pl, commit, rel)
		if err != nil {
			return nil, "", err
		}
		switch {
		case hit == nil:
			return nil, "", noSide()
		case !old && hit.untracked:
			spec = c.Parents[2] + ":" + rel
		case !old:
			spec = c.ID + ":" + rel
		case hit.untracked || len(c.Parents) == 0:
			return nil, "", noSide()
		case hit.OrigPath != "":
			spec = c.Parents[0] + ":" + hit.OrigPath
		default:
			spec = c.Parents[0] + ":" + rel
		}
	} else {
		p, err := pl.relPath(rel)
		if err != nil {
			return nil, "", err
		}
		if p == pl.prefix {
			return nil, "", apiErr("BAD_REQUEST", "Name one image file.")
		}
		name = p
		switch {
		case staged && old:
			orig, err := s.renamedFrom(ctx, pl, p)
			if err != nil {
				return nil, "", err
			}
			if orig == "" {
				orig = p
			}
			spec = "HEAD:" + orig
		case old || staged:
			spec = ":0:" + p // the staged version
		default:
			return workImage(filepath.Join(pl.top, filepath.FromSlash(p)), p)
		}
	}
	out, _, err := s.runX(ctx, pl.top, false, nil, files.MaxImage, nil, "cat-file", "blob", spec)
	if err != nil {
		var re *runError
		if asRun(err, &re) && re.tooLarge {
			return nil, "", tooBig()
		}
		if asRun(err, &re) && re.exit > 0 {
			return nil, "", noSide()
		}
		return nil, "", s.failure(err)
	}
	return typed(out, name)
}

// workImage reads the image in the working tree. A link or a folder is refused.
func workImage(full, name string) ([]byte, string, error) {
	f, err := os.OpenFile(full, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, "", noSide()
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return nil, "", noSide()
	}
	if info.Size() > files.MaxImage {
		return nil, "", tooBig()
	}
	out, err := io.ReadAll(io.LimitReader(f, files.MaxImage+1))
	if err != nil {
		return nil, "", noSide()
	}
	return typed(out, name)
}

func typed(out []byte, name string) ([]byte, string, error) {
	ct := files.ImageType(name, out[:min(len(out), 4096)])
	if ct == "" {
		return nil, "", apiErr("NOT_AN_IMAGE", "This is not an image Mooring can show.")
	}
	return out, ct, nil
}

func noSide() error {
	return apiErr("NOT_FOUND", "This version of the file does not exist.")
}

func tooBig() error {
	return apiErr("TOO_LARGE", "The image is over 50 MB, which is more than Mooring shows.")
}

// httpImage serves one side of an image diff for an <img>, with the raw file route's headers.
func (s *Service) httpImage(w http.ResponseWriter, r *http.Request) {
	switch r.Header.Get("Sec-Fetch-Site") {
	case "", "same-origin", "none":
	default:
		reply(w, http.StatusForbidden, map[string]any{"ok": false, "error": apiErr("OUTSIDE_PROJECT", "This image can only be opened from Mooring.")})
		return
	}
	q := r.URL.Query()
	out, ct, err := s.Image(r.Context(), q.Get("project"), q.Get("folder"), q.Get("path"), q.Get("commit"), q.Get("staged") == "1", q.Get("side") == "old")
	if err != nil {
		s.fail(w, err)
		return
	}
	h := w.Header()
	h.Set("Content-Type", ct)
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; sandbox")
	h.Set("Cache-Control", "no-store")
	h.Set("Content-Length", strconv.Itoa(len(out)))
	if r.Method == http.MethodHead {
		return
	}
	_, _ = w.Write(out)
}
