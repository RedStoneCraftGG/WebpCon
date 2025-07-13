# WebP Converter

A simple tool for mass converting images (JPG, PNG, BMP, GIF, TIFF) to WebP in project folders. It supports automatic backup of original files and a revert feature (restoring original files from backup).

I use it for mass conversion of my project files (mostly Vite and React.js). Instead of discarding them, it's better to keep them.

## Main Features

- Converts all images in the folder to WebP (including subfolders)
- Automatically backs up original files to .webpcon_backup
- Revert feature: restores original files and deletes WebP files
- Supports Windows & Linux (cross-platform)
- Fast process, without external C libraries

## Installation

### Windows

```
go mod tidy
go build -o webcon.exe main.go
```

or run `build.bat`

### Linux

```sh
go mod tidy
go build -o webcon main.go
```

or run `build.sh`

## Usage

### Convert to WebP

```
webcon <project-folder>
```

### Revert

```
webcon <project-folder> revert
```

*Note*: Backup files will be saved in `.webcon_backup`

## Known Issue

For the `.gif` format, it will be converted to a static image on the first frame (limitation of go-native library).