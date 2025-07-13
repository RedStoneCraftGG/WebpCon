package main

import (
	"bufio"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/chai2010/webp"
	"golang.org/x/image/bmp"
	"golang.org/x/image/tiff"
)

var imageExt = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".bmp":  true,
	".gif":  true, // Will be converted into static image
	".tiff": true,
}

var skipDirs = map[string]bool{
	"node_modules":    true,
	".git":            true,
	".webpcon_backup": true,
	"dist":            true,
	// Add another excluded folder if available
}

var skipFiles = map[string]bool{
	"favicon.ico":       true,
	"icon-192x192.png":  true,
	"icon-512x512.png":  true,
	"icon-template.svg": true,
	// Add another if there's something you want to be excluded
}

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		fmt.Println("Usage:")
		fmt.Println("  webpcon <project-path>	# Convert to WebP")
		fmt.Println("  webpcon <project-path> revert	# Revert to original")
		return
	}

	path := args[0]
	if !isSafePath(path) {
		fmt.Println("⚠️  Path is too broad or suspicious. Operation cancelled.")
		return
	}

	if len(args) > 1 && args[1] == "revert" {
		err := revertImages(path)
		if err != nil {
			log.Fatal(err)
		}
		return
	}

	err := convertImages(path)
	if err != nil {
		log.Fatal(err)
	}
}

func isSafePath(path string) bool {
	abs, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	if abs == "/" || len(abs) <= 3 {
		fmt.Println("Path appears to be root or drive (", abs, ")")
		return confirm()
	}

	projectFiles := []string{"package.json", "vite.config.ts", "index.html"}
	found := false
	for _, f := range projectFiles {
		if _, err := os.Stat(filepath.Join(abs, f)); err == nil {
			found = true
			break
		}
	}

	relParts := strings.Split(filepath.ToSlash(abs), "/")
	if len(relParts) > 10 && !found {
		fmt.Printf("Folder is too deep (%d level) and no project files found.\n", len(relParts))
		return confirm()
	}

	return true
}

func confirm() bool {
	fmt.Print("Continue? (y/N): ")
	scan := bufio.NewScanner(os.Stdin)
	if scan.Scan() {
		ans := strings.ToLower(scan.Text())
		return ans == "y" || ans == "yes"
	}
	return false
}

func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = destFile.ReadFrom(sourceFile)
	return err
}

func convertImages(root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if skipDirs[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}

		if skipFiles[info.Name()] {
			fmt.Println("⏭️ Skipping excluded file:", path)
			return nil
		}

		ext := strings.ToLower(filepath.Ext(info.Name()))
		if !imageExt[ext] || ext == ".webp" {
			return nil
		}

		fmt.Println("🔄 Converting:", path)

		relPath, err := filepath.Rel(root, path)
		if err != nil {
			fmt.Printf("❌ Error getting relative path for %s: %v\n", path, err)
			return err
		}

		bakPath := filepath.Join(root, ".webpcon_backup", relPath)
		bakDir := filepath.Dir(bakPath)
		if err := os.MkdirAll(bakDir, 0755); err != nil {
			fmt.Printf("❌ Error creating backup directory %s: %v\n", bakDir, err)
			return err
		}

		// Move the original file to backup
		if err := os.Rename(path, bakPath); err != nil {
			fmt.Printf("❌ Error moving %s to backup: %v\n", path, err)
			return err
		}
		fmt.Printf("💾 Moved to backup: %s\n", relPath)

		// Open the backup file for conversion
		in, err := os.Open(bakPath)
		if err != nil {
			fmt.Printf("❌ Error opening backup file %s: %v\n", bakPath, err)
			return err
		}
		defer in.Close()

		var img image.Image
		switch ext {
		case ".jpg", ".jpeg":
			img, err = jpeg.Decode(in)
		case ".png":
			img, err = png.Decode(in)
		case ".bmp":
			img, err = bmp.Decode(in)
		case ".gif":
			img, err = gif.Decode(in)
		case ".tiff":
			img, err = tiff.Decode(in)
		default:
			return nil
		}
		if err != nil {
			fmt.Printf("❌ Error decoding image %s: %v\n", bakPath, err)
			return err
		}

		webpPath := path[:len(path)-len(ext)] + ".webp"
		outFile, err := os.Create(webpPath)
		if err != nil {
			fmt.Printf("❌ Error creating WebP file %s: %v\n", webpPath, err)
			return err
		}
		defer outFile.Close()

		if err := webp.Encode(outFile, img, &webp.Options{Quality: 80}); err != nil {
			fmt.Printf("❌ Error encoding WebP for %s: %v\n", bakPath, err)
			return err
		}

		fmt.Printf("✅ Converted: %s -> %s\n", relPath, filepath.Base(webpPath))
		return nil
	})
}

func revertImages(root string) error {
	backupRoot := filepath.Join(root, ".webpcon_backup")
	return filepath.Walk(backupRoot, func(bakPath string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(info.Name()))
		if !imageExt[ext] {
			return nil
		}

		relPath, err := filepath.Rel(backupRoot, bakPath)
		if err != nil {
			fmt.Printf("❌ Error getting relative path for %s: %v\n", bakPath, err)
			return err
		}
		origPath := filepath.Join(root, relPath)
		webpPath := origPath[:len(origPath)-len(ext)] + ".webp"

		// Check if the .webp file exists
		if _, err := os.Stat(webpPath); err == nil {
			if err := os.Remove(webpPath); err != nil {
				fmt.Printf("❌ Failed to delete %s: %v\n", webpPath, err)
				return err
			}
			fmt.Printf("🗑️  Deleted: %s\n", webpPath)
		}

		// Restore the backup file to its original location
		origDir := filepath.Dir(origPath)
		if err := os.MkdirAll(origDir, 0755); err != nil {
			fmt.Printf("❌ Error creating directory %s: %v\n", origDir, err)
			return err
		}
		if err := copyFile(bakPath, origPath); err != nil {
			fmt.Printf("❌ Error restoring %s: %v\n", origPath, err)
			return err
		}
		fmt.Printf("✅ Restored: %s\n", relPath)
		return nil
	})
}
