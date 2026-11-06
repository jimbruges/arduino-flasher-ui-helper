package provision

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/klauspost/compress/zstd"
)

var numberedXML = regexp.MustCompile(`^(rawprogram|patch)([0-9]+)\.xml$`)

func findFlashDir(root string) (string, error) {
	var found []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type()&os.ModeSymlink != 0 {
			return errors.New("Factory folders must not contain symbolic links.")
		}
		if d.IsDir() {
			if _, err := os.Stat(filepath.Join(path, "rawprogram0.xml")); err == nil {
				found = append(found, path)
			}
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if len(found) != 1 {
		return "", fmt.Errorf("Expected one factory flash folder, found %d.", len(found))
	}
	return found[0], nil
}
func safePath(root, name string) (string, error) {
	if strings.Contains(name, "\\") || filepath.IsAbs(name) {
		return "", fmt.Errorf("Unsafe image path %q", name)
	}
	clean := filepath.Clean(name)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("Unsafe image path %q", name)
	}
	return filepath.Join(root, clean), nil
}

// Extraction never follows archive links and bounds disk use and entry count.
func extractArchive(ctx context.Context, path, dest string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	var total int64
	entries := 0
	save := func(name string, size int64, dir bool, r io.Reader) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		entries++
		if entries > 100000 || size < 0 || size > (256<<30)-total {
			return errors.New("Factory archive exceeds extraction limits.")
		}
		total += size
		target, err := safePath(dest, name)
		if err != nil {
			return err
		}
		if dir {
			return os.MkdirAll(target, 0700)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0600)
		if err != nil {
			return err
		}
		_, copyErr := io.CopyN(out, &contextReader{ctx, r}, size)
		closeErr := out.Close()
		return errors.Join(copyErr, closeErr)
	}
	if strings.HasSuffix(strings.ToLower(path), ".zip") {
		stat, err := f.Stat()
		if err != nil {
			return err
		}
		z, err := zip.NewReader(f, stat.Size())
		if err != nil {
			return err
		}
		for _, entry := range z.File {
			if entry.Mode()&os.ModeSymlink != 0 {
				return errors.New("Archive links are unsupported.")
			}
			if entry.UncompressedSize64 > 256<<30 {
				return errors.New("Archive entry too large.")
			}
			r, err := entry.Open()
			if err != nil {
				return err
			}
			err = save(entry.Name, int64(entry.UncompressedSize64), entry.FileInfo().IsDir(), r)
			r.Close()
			if err != nil {
				return err
			}
		}
		return nil
	}
	var reader io.Reader = f
	if strings.HasSuffix(path, ".zst") {
		z, err := zstd.NewReader(f, zstd.WithDecoderMaxMemory(256<<20))
		if err != nil {
			return err
		}
		defer z.Close()
		reader = z
	} else if strings.HasSuffix(path, ".gz") || strings.HasSuffix(path, ".tgz") {
		z, err := gzip.NewReader(f)
		if err != nil {
			return err
		}
		defer z.Close()
		reader = z
	}
	tr := tar.NewReader(reader)
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		switch h.Typeflag {
		case tar.TypeDir:
			err = save(h.Name, 0, true, tr)
		case tar.TypeReg, tar.TypeRegA:
			err = save(h.Name, h.Size, false, tr)
		default:
			return fmt.Errorf("Unsupported archive entry %q", h.Name)
		}
		if err != nil {
			return err
		}
	}
}

type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r *contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}
func validateFactory(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var programs, patches []string
	for _, entry := range entries {
		m := numberedXML.FindStringSubmatch(entry.Name())
		if m == nil {
			continue
		}
		if m[1] == "rawprogram" {
			programs = append(programs, entry.Name())
			if _, err := os.Stat(filepath.Join(dir, "patch"+m[2]+".xml")); err != nil {
				return nil, fmt.Errorf("Missing patch file for %s", entry.Name())
			}
		} else {
			patches = append(patches, entry.Name())
		}
	}
	if len(programs) == 0 {
		return nil, errors.New("No rawprogram XML files found.")
	}
	sort.Strings(programs)
	sort.Strings(patches)
	files := append(programs, patches...)
	for _, name := range append([]string{"prog_firehose_ddr.elf"}, files...) {
		stat, err := os.Stat(filepath.Join(dir, name))
		if err != nil || !stat.Mode().IsRegular() {
			return nil, fmt.Errorf("Missing factory file %s", name)
		}
	}
	for _, name := range files {
		f, err := os.Open(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		dec := xml.NewDecoder(f)
		for {
			token, err := dec.Token()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				f.Close()
				return nil, err
			}
			element, ok := token.(xml.StartElement)
			if !ok {
				continue
			}
			for _, attr := range element.Attr {
				if attr.Name.Local != "filename" || attr.Value == "" || attr.Value == "DISK" {
					continue
				}
				path, err := safePath(dir, attr.Value)
				if err != nil {
					f.Close()
					return nil, err
				}
				stat, err := os.Stat(path)
				if err != nil || !stat.Mode().IsRegular() {
					f.Close()
					return nil, fmt.Errorf("%s references missing payload %s", name, attr.Value)
				}
			}
		}
		f.Close()
	}
	return files, nil
}

// Copy small/mutable metadata; share read-only large payloads using hard links.
func privateFactory(ctx context.Context, source, dest string) error {
	return filepath.WalkDir(source, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dest, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0700)
		}
		if !d.Type().IsRegular() {
			return fmt.Errorf("Unsupported factory file %s", rel)
		}
		// GPT and XML files may be modified by the upstream flasher.
		mutable := strings.HasSuffix(strings.ToLower(rel), ".xml") || strings.Contains(strings.ToLower(rel), "gpt")
		if !mutable {
			if err := os.Link(path, target); err == nil {
				return nil
			}
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0600)
		if err != nil {
			return err
		}
		_, err = io.Copy(out, &contextReader{ctx, in})
		return errors.Join(err, out.Close())
	})
}
func (e *Engine) runFactory(ctx context.Context, r Request, p Plan, logDir string) ([]Result, error) {
	work, err := os.MkdirTemp("", "arduino-factory-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(work)
	e.event(Event{Kind: "phase", Phase: "Prepare factory image", Message: "Preparing one shared factory image."})
	image := r.Image
	if image == "" {
		download := filepath.Join(work, "download")
		if err := os.Mkdir(download, 0700); err != nil {
			return nil, err
		}
		args := []string{"download", r.Board, "--dest-dir", download}
		if r.Version != "" {
			args = append(args, "--version", r.Version)
		}
		if r.OS != "" {
			args = append(args, "--os", r.OS)
		}
		if _, err := runCommand(ctx, e.adapter.CLI, args, "", func(line string) { e.event(ProgressEvent(line, "image")) }); err != nil {
			return nil, err
		}
		entries, err := os.ReadDir(download)
		if err != nil {
			return nil, err
		}
		if len(entries) != 1 {
			return nil, errors.New("Download did not produce exactly one factory archive.")
		}
		image = filepath.Join(download, entries[0].Name())
	}
	stat, err := os.Stat(image)
	if err != nil {
		return nil, err
	}
	if !stat.IsDir() {
		dest := filepath.Join(work, "extracted")
		if err := os.Mkdir(dest, 0700); err != nil {
			return nil, err
		}
		if err := extractArchive(ctx, image, dest); err != nil {
			return nil, err
		}
		image = dest
	}
	flash, err := findFlashDir(image)
	if err != nil {
		return nil, err
	}
	xmls, err := validateFactory(flash)
	if err != nil {
		return nil, err
	}
	if bytes, ok := factoryProgramSpan(flash, xmls); ok {
		e.event(Event{Kind: "data", Data: fmt.Sprintf("%.2f GiB programmed per board · %.2f GiB total (%d targets; USB bytes vary for sparse images)", float64(bytes)/(1<<30), float64(bytes)*float64(len(p.Targets))/(1<<30), len(p.Targets))})
	}
	// Always snapshot the source so upstream work cannot change the user's files.
	shared := filepath.Join(work, "shared")
	if err := copyFactory(ctx, flash, shared); err != nil {
		return nil, err
	}
	results := make([]Result, len(p.Targets))
	queue := make(chan int)
	var wg sync.WaitGroup
	for worker := 0; worker < p.Jobs; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for index := range queue {
				serial := p.Targets[index]
				started := time.Now()
				result := Result{Serial: serial, Status: "failed"}
				targetCtx, cancel := context.WithTimeout(ctx, time.Duration(r.TimeoutMinutes)*time.Minute)
				root := filepath.Join(work, serial)
				private := filepath.Join(root, "flash")
				err := privateFactory(targetCtx, shared, private)
				if err == nil {
					path := e.adapter.CLI
					args := []string{"flash", r.Board, root, "--serial", serial, "--yes"}
					cwd := ""
					if r.Board == "ventunoq" {
						path = e.adapter.QDL
						args = append([]string{"--serial", serial[2:], "--storage", "emmc", "prog_firehose_ddr.elf"}, xmls...)
						cwd = private
					}
					e.event(Event{Kind: "phase", Serial: serial, Phase: "Write factory partitions"})
					_, err = runCommand(targetCtx, path, args, cwd, func(line string) { e.event(ProgressEvent(line, serial)) })
				}
				if err == nil {
					result.Status = "flashed"
				} else {
					result.Error = err.Error()
					if targetCtx.Err() != nil {
						result.Status = "cancelled"
					}
				}
				cancel()
				result.Seconds = time.Since(started).Seconds()
				results[index] = result
				e.event(Event{Kind: "result", Serial: serial, Status: result.Status, Message: result.Error})
				os.RemoveAll(root)
			}
		}()
	}
	for i := range p.Targets {
		queue <- i
	}
	close(queue)
	wg.Wait()
	for _, result := range results {
		if result.Status != "flashed" {
			return results, errors.New("Some boards did not finish. See individual results.")
		}
	}
	return results, nil
}
func copyFactory(ctx context.Context, source, dest string) error {
	return filepath.WalkDir(source, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dest, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0700)
		}
		if !d.Type().IsRegular() {
			return errors.New("Factory images must contain regular files only.")
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		_, err = io.Copy(out, &contextReader{ctx, in})
		return errors.Join(err, out.Close())
	})
}

// Declared program ranges express storage scale, not compressed or sparse USB bytes.
func factoryProgramSpan(dir string, names []string) (uint64, bool) {
	var total uint64
	for _, name := range names {
		if !strings.HasPrefix(name, "rawprogram") {
			continue
		}
		f, err := os.Open(filepath.Join(dir, name))
		if err != nil {
			return 0, false
		}
		dec := xml.NewDecoder(f)
		for {
			token, err := dec.Token()
			if err == io.EOF {
				break
			}
			if err != nil {
				f.Close()
				return 0, false
			}
			element, ok := token.(xml.StartElement)
			if !ok || element.Name.Local != "program" {
				continue
			}
			attrs := map[string]string{}
			for _, attr := range element.Attr {
				attrs[attr.Name.Local] = attr.Value
			}
			if attrs["filename"] == "" {
				continue
			}
			sectors, err1 := strconv.ParseUint(attrs["num_partition_sectors"], 10, 64)
			sectorSize, err2 := strconv.ParseUint(attrs["SECTOR_SIZE_IN_BYTES"], 10, 64)
			if err1 != nil || err2 != nil || sectorSize == 0 || sectors > (^uint64(0)-total)/sectorSize {
				f.Close()
				return 0, false
			}
			total += sectors * sectorSize
		}
		f.Close()
	}
	return total, total > 0
}
