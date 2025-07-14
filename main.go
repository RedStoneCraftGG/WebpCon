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

	"image/draw"

	"github.com/HugoSmits86/nativewebp"
	"github.com/chai2010/webp"
	"golang.org/x/image/bmp"
	"golang.org/x/image/tiff"
)

var imageExt = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".bmp":  true,
	".gif":  true, // Will be converted into static image. gif conversion still in experiment
	".tiff": true,
}

var skipDirs = map[string]bool{
	"node_modules":    true,
	".git":            true,
	".webpcon_backup": true,
	".webcon_cache":   true,
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
		fmt.Println("  webpcon <project-path>\t# Convert to WebP")
		fmt.Println("  webpcon <project-path> revert\t# Revert to original")
		return
	}

	path := args[0]
	if !isSafePath(path) {
		fmt.Println("⚠️  Path is too broad or suspicious. Operation cancelled.")
		return
	}

	enableGif := false
	for _, arg := range args {
		if arg == "--enable-gif" || arg == "--gif" {
			enableGif = true
		}
	}

	if len(args) > 1 && args[1] == "revert" {
		err := revertImages(path)
		if err != nil {
			log.Fatal(err)
		}
		return
	}

	err := convertImages(path, enableGif)
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

	projectFiles := []string{"package.json", "vite.config.ts", "vite.config.js", "next.config.js", "tsconfig.json", "vue.config.js", "nuxt.config.ts", "nuxt.config.js", "tsconfig.json", "jsconfig.json", "babel.config.js", "postcss.config.js", "tailwind.config.js", "angular.json", "svelte.config.js", "index.html"} // Add another if you want
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

func convertImages(root string, enableGif bool) error {
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

		if err := os.Rename(path, bakPath); err != nil {
			fmt.Printf("❌ Error moving %s to backup: %v\n", path, err)
			return err
		}
		fmt.Printf("💾 Moved to backup: %s\n", relPath)

		in, err := os.Open(bakPath)
		if err != nil {
			fmt.Printf("❌ Error opening backup file %s: %v\n", bakPath, err)
			return err
		}
		defer in.Close()

		var img image.Image
		var gifFrames *gif.GIF
		switch ext {
		case ".jpg", ".jpeg":
			img, err = jpeg.Decode(in)
		case ".png":
			img, err = png.Decode(in)
		case ".bmp":
			img, err = bmp.Decode(in)
		case ".gif":
			if enableGif {
				gifFrames, err = gif.DecodeAll(in)
				if err == nil && len(gifFrames.Image) > 1 {
					cacheDir := filepath.Join(root, ".webcon_cache")
					if err := gifExtractor(bakPath, cacheDir); err != nil {
						fmt.Printf("❌ Error extracting GIF frame: %v\n", err)
						return err
					}
					for i := range gifFrames.Image {
						pngPath := filepath.Join(cacheDir, fmt.Sprintf("frame_%02d.png", i))
						webpPath := filepath.Join(cacheDir, fmt.Sprintf("frame_%02d.webp", i))
						err := frameCompress(pngPath, webpPath, 60)
						if err != nil {
							fmt.Printf("❌ Error compressing frame to WebP (frame %d): %v\n", i, err)
							return err
						}
					}
					webpPath := path[:len(path)-len(ext)] + ".webp"
					err := buildAnimatedWebp(
						cacheDir,
						webpPath,
						func() []uint {
							d := make([]uint, len(gifFrames.Delay))
							for i, v := range gifFrames.Delay {
								d[i] = uint(v) * 10
							}
							return d
						}(),
						func() []uint {
							d := make([]uint, len(gifFrames.Disposal))
							for i, v := range gifFrames.Disposal {
								d[i] = uint(v)
							}
							return d
						}(),
						uint16(gifFrames.LoopCount),
						0xffffffff,
					)
					if err != nil {
						fmt.Printf("❌ Error build animated WebP: %v\n", err)
						return err
					}
					deleteCache(cacheDir)
					fmt.Printf("✅ Converted (experimental): %s -> %s\n", relPath, filepath.Base(webpPath))
					return nil
				} else {
					img, err = gif.Decode(in)
				}
			} else {
				img, err = gif.Decode(in)
			}
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

		if _, err := os.Stat(webpPath); err == nil {
			if err := os.Remove(webpPath); err != nil {
				fmt.Printf("❌ Failed to delete %s: %v\n", webpPath, err)
				return err
			}
			fmt.Printf("🗑️  Deleted: %s\n", webpPath)
		}

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

// Helpers
func gifExtractor(gifPath, cacheDir string) error {
	f, err := os.Open(gifPath)
	if err != nil {
		return err
	}
	defer f.Close()

	gifFrames, err := gif.DecodeAll(f)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return err
	}

	for i, frame := range gifFrames.Image {
		rgba := image.NewRGBA(frame.Bounds())
		draw.Draw(rgba, frame.Bounds(), frame, image.Point{}, draw.Over)
		framePath := filepath.Join(cacheDir, fmt.Sprintf("frame_%02d.png", i))
		out, err := os.Create(framePath)
		if err != nil {
			return err
		}
		err = png.Encode(out, rgba)
		out.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func frameCompress(pngPath, webpPath string, quality float32) error {
	f, err := os.Open(pngPath)
	if err != nil {
		return err
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		return err
	}
	out, err := os.Create(webpPath)
	if err != nil {
		return err
	}
	defer out.Close()
	return webp.Encode(out, img, &webp.Options{Quality: quality})
}

func buildAnimatedWebp(framesDir, outPath string, durations []uint, disposals []uint, loopCount uint16, bgColor uint32) error {
	frameCount := len(durations)
	var images []image.Image
	for i := 0; i < frameCount; i++ {
		webpPath := filepath.Join(framesDir, fmt.Sprintf("frame_%02d.webp", i))
		f, err := os.Open(webpPath)
		if err != nil {
			return err
		}
		img, err := webp.Decode(f)
		f.Close()
		if err != nil {
			return err
		}
		images = append(images, img)
	}
	ani := nativewebp.Animation{
		Images:          images,
		Durations:       durations,
		Disposals:       disposals,
		LoopCount:       loopCount,
		BackgroundColor: bgColor,
	}
	out, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer out.Close()
	return nativewebp.EncodeAll(out, &ani, nil)
}

func deleteCache(cacheDir string) error {
	return os.RemoveAll(cacheDir)
}
