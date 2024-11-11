package main

import (
	"bufio"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type Command string

const (
	Init    Command = "init"
	Status  Command = "status"
	Add     Command = "add"
	Help    Command = "help"
	Unknown Command = "unknown"
)

func parseCommand(cmd string) Command {
	switch strings.ToLower(cmd) {
	case "init":
		return Init
	case "status":
		return Status
	case "add":
		return Add
	case "help":
		return Help
	default:
		return Unknown
	}
}

func getDirectory(args []string) (string, error) {
	if len(args) < 1 {
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("error getting current directory: %v", err)
		}
		return cwd, nil
	}
	return args[0], nil
}

func initRepository(dir string) error {
	trackrDir := filepath.Join(dir, ".trackr")
	if _, err := os.Stat(trackrDir); err == nil {
		return fmt.Errorf("trackr repository already initialized")
	}

	subDirs := []string{"objects", "refs/heads", "refs/tags", "hooks", "info"}
	for _, subDir := range subDirs {
		path := filepath.Join(trackrDir, subDir)

		if err := os.MkdirAll(path, 0755); err != nil {
			return fmt.Errorf("error creating directory %s: %v", path, err)
		}
	}

	files := map[string]string{
		"HEAD":        "ref: refs/heads/master\n",
		"config":      "[core]\n  repositoryformatversion = 0\n  filemode = true\n  bare = false\n",
		"description": "Unnamed repository; edit this file to name it.\n",
	}

	for fileName, content := range files {
		filePath := filepath.Join(trackrDir, fileName)

		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			return fmt.Errorf("error creating %s: %v", fileName, err)
		}
	}

	ignorePath := filepath.Join(dir, ".trackrignore")
	initialIgnoreContent := "# Add patterns to ignore\n# Example:\n# *.log\n# temp/\n.git/\n"
	if err := os.WriteFile(ignorePath, []byte(initialIgnoreContent), 0644); err != nil {
		return fmt.Errorf("error creating .trackrignore: %v", err)
	}

	fmt.Println("Initialized empty repository in", trackrDir)
	return nil
}

func hashObject(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("error opening file %s: %v", filePath, err)
	}

	defer file.Close()

	hasher := sha256.New()
	content, err := io.ReadAll(file)
	if err != nil {
		return "", fmt.Errorf("error reading file %s: %v", filePath, err)
	}

	header := fmt.Sprintf("blob %d\x00", len(content))
	hasher.Write([]byte(header))
	hasher.Write(content)

	hash := fmt.Sprintf("%x", hasher.Sum(nil))

	return hash, nil
}

func storeObject(dir, hash string, content []byte) error {
	objectDir := filepath.Join(dir, ".trackr", "objects", hash[:2])
	objectPath := filepath.Join(objectDir, hash[2:])

	if _, err := os.Stat(objectPath); err == nil {
		return nil
	}

	if err := os.MkdirAll(objectDir, 0755); err != nil {
		return fmt.Errorf("error creating object directory: %v", err)
	}

	if err := os.WriteFile(objectPath, content, 0644); err != nil {
		return fmt.Errorf("error writing object file: %v", err)
	}

	return nil
}

func addToIndex(dir, filePath, hash string) error {
	indexPath := filepath.Join(dir, ".trackr", "index")

	file, err := os.OpenFile(indexPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("error opening index file: %v", err)
	}

	defer file.Close()

	line := fmt.Sprintf("%s %s\n", hash, filePath)
	if _, err := file.WriteString(line); err != nil {
		return fmt.Errorf("error writing to index file: %v", err)
	}

	return nil
}

func addFile(dir, filePath string, ignorePatterns []string) error {
	fullPath := filepath.Join(dir, filePath)

	matched, err := isIgnored(filePath, ignorePatterns)
	if err != nil {
		return err
	}

	if matched {
		return nil
	}

	info, err := os.Stat(fullPath)
	if os.IsNotExist(err) {
		return fmt.Errorf("file %s does not exist", filePath)
	}

	if info.IsDir() {
		return fmt.Errorf("'%s' is a directory", filePath)
	}

	hash, err := hashObject(fullPath)
	if err != nil {
		return err
	}

	content, err := os.ReadFile(fullPath)
	if err != nil {
		return fmt.Errorf("error reading file %s: %v", filePath, err)
	}

	if err := storeObject(dir, hash, content); err != nil {
		return err
	}

	if err := addToIndex(dir, filePath, hash); err != nil {
		return err
	}

	fmt.Printf("Added %s to the index.\n", filePath)
	return nil
}

func readIndex(dir string) (map[string]string, error) {
	indexPath := filepath.Join(dir, ".trackr", "index")
	index := make(map[string]string)

	file, err := os.Open(indexPath)
	if err != nil {
		if os.IsNotExist(err) {
			return index, nil
		}
		return nil, fmt.Errorf("error opening index file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			continue
		}
		index[parts[1]] = parts[0]
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading index file: %v", err)
	}

	return index, nil
}

func readIgnorePatterns(dir string) ([]string, error) {
	ignorePath := filepath.Join(dir, ".trackrignore")
	file, err := os.Open(ignorePath)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil // No ignore file, return empty slice
		}
		return nil, fmt.Errorf("error opening .trackrignore: %v", err)
	}
	defer file.Close()

	var patterns []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, line)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading .trackrignore: %v", err)
	}

	return patterns, nil
}

func isIgnored(filePath string, patterns []string) (bool, error) {
	for _, pattern := range patterns {
		if strings.HasSuffix(pattern, "/") {
			dirPattern := strings.TrimSuffix(pattern, "/")
			if strings.HasPrefix(filePath, dirPattern+"/") || filePath == dirPattern {
				return true, nil
			}
		} else {
			match, err := filepath.Match(pattern, filepath.Base(filePath))
			if err != nil {
				return false, fmt.Errorf("invalid pattern %s: %v", pattern, err)
			}
			if match {
				return true, nil
			}
		}
	}
	return false, nil
}

func addAllFiles(dir string, ignorePatterns []string) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() && info.Name() == ".trackr" {
			return filepath.SkipDir
		}

		if !info.IsDir() {
			relPath, err := filepath.Rel(dir, path)
			if err != nil {
				return err
			}
			return addFile(dir, relPath, ignorePatterns)
		}
		return nil
	})
}

func checkStatus(dir string) error {
	trackrDir := filepath.Join(dir, ".trackr")

	// Check if .trackr exists
	if _, err := os.Stat(trackrDir); os.IsNotExist(err) {
		return fmt.Errorf("not a Trackr repository (or any of the parent directories)")
	}

	// Read HEAD to find the current branch
	headPath := filepath.Join(trackrDir, "HEAD")
	headContent, err := os.ReadFile(headPath)
	if err != nil {
		return fmt.Errorf("error reading HEAD: %v", err)
	}

	headLines := strings.Split(string(headContent), "\n")
	if len(headLines) == 0 {
		return fmt.Errorf("invalid HEAD file")
	}

	refLine := headLines[0]
	if !strings.HasPrefix(refLine, "ref: ") {
		return fmt.Errorf("invalid HEAD reference")
	}

	currentRef := strings.TrimPrefix(refLine, "ref: ")
	currentBranch := filepath.Base(currentRef)

	indexMap, err := readIndex(dir)
	if err != nil {
		return err
	}

	ignorePatterns, err := readIgnorePatterns(dir)
	if err != nil {
		return err
	}

	presentFiles := make(map[string]bool)
	untrackedFiles := []string{}
	modifiedFiles := []string{}
	deletedFiles := []string{}

	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() && info.Name() == ".trackr" {
			return filepath.SkipDir
		}

		if !info.IsDir() {
			relPath, err := filepath.Rel(dir, path)
			if err != nil {
				return err
			}

			ignored, err := isIgnored(relPath, ignorePatterns)
			if err != nil {
				return err
			}
			if ignored {
				return nil
			}

			presentFiles[relPath] = true

			if hash, exists := indexMap[relPath]; exists {
				currentHash, err := hashObject(path)
				if err != nil {
					return err
				}
				if currentHash != hash {
					modifiedFiles = append(modifiedFiles, relPath)
				}
			} else {
				untrackedFiles = append(untrackedFiles, relPath)
			}
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("error walking the file tree: %v", err)
	}

	// Identify deleted files
	for file := range indexMap {
		if !presentFiles[file] {
			deletedFiles = append(deletedFiles, file)
		}
	}

	// Display status
	fmt.Printf("On branch %s\n", currentBranch)

	if len(indexMap) > 0 {
		fmt.Println("Changes to be committed:")
		for file := range indexMap {
			fmt.Printf("\033[32m  new file:   %s\033[0m\n", file)
		}
	}

	if len(modifiedFiles) > 0 {
		fmt.Println("\nChanges not staged for commit:")
		for _, file := range modifiedFiles {
			fmt.Printf("  \033[33mmodified:   %s\033[0m\n", file)
		}
	}
	
	if len(deletedFiles) > 0 {
		fmt.Println("\nDeleted files:")
		for _, file := range deletedFiles {
			fmt.Printf("  \033[33mdeleted:    %s\033[0m\n", file)
		}
	}

	if len(untrackedFiles) > 0 {
		fmt.Println("\nUntracked files:")
		fmt.Println("  (use \"trackr add <file>...\" to include in what will be committed)")
		for _, file := range untrackedFiles {
			fmt.Printf("    \033[31m%s\033[0m\n", file)
		}
	}

	if len(indexMap) == 0 && len(modifiedFiles) == 0 && len(deletedFiles) == 0 && len(untrackedFiles) == 0 {
		fmt.Println("No changes.")
	}

	return nil
}

func printHelp() {
	helpText := `Trackr - A Simple Version Control System

Usage:
  trackr <command> [arguments]

Available Commands:
  init       Initialize a new Trackr repository
  add        Add file contents to the index
  status     Show the working tree status
  help       Display this help message

Use "trackr help <command>" for more information about a command.
`

	fmt.Println(helpText)
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: trackr <command> [<args>]")
		fmt.Println("Run 'trackr help' to see available commands.")
		return
	}

	commandStr := os.Args[1]
	command := parseCommand(commandStr)
	args := os.Args[2:]

	switch command {
	case Init:
		dir, err := getDirectory(args)
		if err != nil {
			fmt.Println("Error:", err)
			return
		}

		if err := initRepository(dir); err != nil {
			fmt.Println("Error initializing repository:", err)
		}

	case Add:
		if len(args) < 1 {
			fmt.Println("Usage: trackr add <file> or trackr add .")
			return
		}
		target := args[0]
		dir, err := getDirectory(args[1:])
		if err != nil {
			fmt.Println("Error:", err)
			return
		}

		ignorePatterns, err := readIgnorePatterns(dir)
		if err != nil {
			fmt.Println("Error reading .trackrignore:", err)
			return
		}

		if target == "." {
			if err := addAllFiles(dir, ignorePatterns); err != nil {
				fmt.Println("Error adding files:", err)
			}
		} else {
			if err := addFile(dir, target, ignorePatterns); err != nil {
				fmt.Println("Error adding file:", err)
			}
		}

	case Status:
		dir, err := getDirectory(args)
		if err != nil {
			fmt.Println("Error:", err)
			return
		}

		if err := checkStatus(dir); err != nil {
			fmt.Println("Error:", err)
		}

	case Help:
		printHelp()

	case Unknown:
		fmt.Printf("Unknown command: %s\n", commandStr)
		fmt.Println("Run 'trackr help' to see available commands.")
	}
}
