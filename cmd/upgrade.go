package main

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
)

var version = "dev"

func replaceBinary(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	// Remove the existing binary before writing — on Linux you cannot open a
	// running executable for writing ("text file busy"). Removing it unlinks the
	// old inode while the running process keeps its file descriptor; the new
	// file gets a fresh inode at the same path.
	os.Remove(dest)

	out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

type Release struct {
	TagName string `json:"tag_name"`
}

func upgrade() {
	repo := "tofunmiadewuyi/dbq"

	// Get latest release
	resp, err := http.Get("https://api.github.com/repos/" + repo + "/releases/latest")
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	var rel Release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		fmt.Println("Upgrade failed: could not parse release info:", err)
		return
	}
	if rel.TagName == "" {
		fmt.Println("Upgrade failed: no release found (check https://github.com/" + repo + "/releases)")
		return
	}

	if rel.TagName == version {
		fmt.Println("Already up to date.")
		return
	}

	fmt.Printf("Update available: %s (current: %s)\n", rel.TagName, version)
	fmt.Print("Continue? (y/n): ")

	var input string
	fmt.Scanln(&input)

	if input != "y" && input != "Y" {
		fmt.Println("Aborted.")
		return
	}

	fmt.Println("Upgrading to", rel.TagName)

	osName := runtime.GOOS
	arch := runtime.GOARCH

	filename := fmt.Sprintf("dbq_%s_%s_%s.tar.gz", rel.TagName, osName, arch)
	url := fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", repo, rel.TagName, filename)

	tmpFile := "/tmp/dbq.tar.gz"

	out, err := os.Create(tmpFile)
	if err != nil {
		fmt.Println("Upgrade failed:", err)
		return
	}
	defer out.Close()

	resp, err = http.Get(url)
	if err != nil {
		fmt.Println("Upgrade failed: download error:", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("Upgrade failed: could not download release (HTTP %d)\n", resp.StatusCode)
		return
	}

	if _, err := io.Copy(out, resp.Body); err != nil {
		fmt.Println("Upgrade failed: download error:", err)
		return
	}
	out.Close()

	// Extract
	f, err := os.Open(tmpFile)
	if err != nil {
		fmt.Println("Upgrade failed:", err)
		return
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		fmt.Println("Upgrade failed: could not read archive:", err)
		return
	}
	tr := tar.NewReader(gzr)

	var binPath = "/tmp/dbq_new"
	found := false

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Println("Upgrade failed: could not read archive:", err)
			return
		}
		if hdr.Name == "dbq" {
			outFile, err := os.Create(binPath)
			if err != nil {
				fmt.Println("Upgrade failed:", err)
				return
			}
			io.Copy(outFile, tr)
			outFile.Close()
			found = true
			break
		}
	}

	if !found {
		fmt.Println("Upgrade failed: binary not found in archive")
		return
	}

	os.Chmod(binPath, 0755)

	current, _ := os.Executable()

	err = replaceBinary(binPath, current)
	if err != nil {
		fallback := filepath.Join(os.Getenv("HOME"), ".local/bin/dbq")
		os.MkdirAll(filepath.Dir(fallback), 0755)
		err2 := replaceBinary(binPath, fallback)
		if err2 != nil {
			fmt.Println("Upgrade failed:", err2)
			return
		}
		fmt.Printf("Installed to %s (original location was not writable).\n", fallback)
		fmt.Println("Ensure ~/.local/bin is in your $PATH.")
		return
	}

	fmt.Println("Upgrade complete.")
}
