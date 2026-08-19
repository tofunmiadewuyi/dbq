package main

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

var version = "dev"

func replaceBinary(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	// Write to a temp file in the same directory as dest, then rename into
	// place. rename() swaps directory entries atomically without opening the
	// running executable for writing, avoiding "text file busy" on Linux.
	tmp := dest + ".new"
	out, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	out.Close()

	return os.Rename(tmp, dest)
}

const defaultReleaseBase = "https://dbq.tofunmiadewuyi.com/releases/dbq"

// releaseBase is where artifacts live; DBQ_RELEASE_BASE overrides the default.
func releaseBase() string {
	base := os.Getenv("DBQ_RELEASE_BASE")
	if base == "" {
		base = defaultReleaseBase
	}
	return strings.TrimRight(base, "/")
}

// latestVersion reads the newest tag from the release base's latest.txt.
func latestVersion(base string) (string, error) {
	resp, err := http.Get(base + "/latest.txt")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d fetching latest.txt", resp.StatusCode)
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

func upgrade() {
	base := releaseBase()

	latest, err := latestVersion(base)
	if err != nil {
		fmt.Fprintf(os.Stderr, "upgrade failed: %v\n", err)
		os.Exit(1)
	}
	if latest == "" {
		fmt.Println("Upgrade failed: no release found at", base)
		return
	}

	if latest == version {
		fmt.Println("Already up to date.")
		return
	}

	fmt.Printf("Update available: %s (current: %s)\n", latest, version)
	fmt.Print("Continue? (y/n): ")

	var input string
	fmt.Scanln(&input)

	if input != "y" && input != "Y" {
		fmt.Println("Aborted.")
		return
	}

	fmt.Println("Upgrading to", latest)

	osName := runtime.GOOS
	arch := runtime.GOARCH

	filename := fmt.Sprintf("dbq_%s_%s_%s.tar.gz", latest, osName, arch)
	url := fmt.Sprintf("%s/%s/%s", base, latest, filename)

	tmpFile := "/tmp/dbq.tar.gz"

	out, err := os.Create(tmpFile)
	if err != nil {
		fmt.Println("Upgrade failed:", err)
		return
	}
	defer out.Close()

	resp, err := http.Get(url)
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

func uninstall() {
	path, err := os.Executable()
	if err != nil {
		fmt.Println("Uninstall failed: could not determine binary path:", err)
		os.Exit(1)
	}

	fmt.Printf("This will remove %s. Continue? (y/n): ", path)
	var input string
	fmt.Scanln(&input)
	if input != "y" && input != "Y" {
		fmt.Println("Aborted.")
		return
	}

	if err := os.Remove(path); err != nil {
		fmt.Println("Permission denied. Retrying with sudo...")
		cmd := exec.Command("sudo", "rm", path)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Println("Uninstall failed:", err)
			os.Exit(1)
		}
	}

	fmt.Println("dbq removed.")
}
