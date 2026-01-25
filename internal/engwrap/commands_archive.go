package engwrap

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// TODO: archive with password

// Archive exports the workspace data to a compressed archive
func Archive(name, outputPath string) error {
	workDir, err := GetEngwrapWorkDir()
	if err != nil {
		return err
	}
	workspacePath := filepath.Join(workDir, name)

	if _, err := os.Stat(workspacePath); os.IsNotExist(err) {
		return fmt.Errorf("workspace for '%s' does not exist", name)
	}

	finalPath := outputPath
	if !strings.HasSuffix(finalPath, ".tar.gz") {
		finalPath += ".tar.gz"
	}

	fmt.Printf("Archiving workspace '%s' to '%s'\n", name, finalPath)

	outFile, err := os.Create(finalPath)
	if err != nil {
		return fmt.Errorf("error creating output file: %w", err)
	}
	defer outFile.Close()

	gw := gzip.NewWriter(outFile)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	// Walk workspace and add to tar
	baseDir := filepath.Dir(workspacePath)
	err = filepath.Walk(workspacePath, func(file string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		header, err := tar.FileInfoHeader(fi, fi.Name())
		if err != nil {
			return err
		}

		// Update name to be relative to parent of workspacePath
		relPath, err := filepath.Rel(baseDir, file)
		if err != nil {
			return err
		}
		header.Name = relPath

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if !fi.Mode().IsRegular() {
			return nil
		}

		f, err := os.Open(file)
		if err != nil {
			return err
		}
		defer f.Close()

		if _, err := io.Copy(tw, f); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		os.Remove(finalPath)
		return fmt.Errorf("error creating archive: %w", err)
	}

	fmt.Printf("Workspace archived to: %s\n", finalPath)
	return nil
}
