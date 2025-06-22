package main

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

func main() {
	args := os.Args[1:]
	if len(args) == 0 || len(args) == 1 {
		log.Fatal("directory path and module name are not specified")
	}

	dir := args[0]
	toChange := args[1]

	modName := findModFile(dir, toChange)
	log.Printf("  [MOD NAME]  %s\n", modName)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		reinitializeGitRepo(dir)
		if strings.Contains(toChange, "github.com") {
			repoHttpsLink := strings.Replace(toChange, "github.com", "https://github.com", 1)
			exec.Command("git", "remote", "add", "origin", repoHttpsLink)
		}
	}()
	checkDir(dir, toChange, modName)
	wg.Wait()
}

func reinitializeGitRepo(repoPath string) error {
	gitDirPath := filepath.Join(repoPath, ".git")

	log.Printf("  [REMOVE GIT]  %s", gitDirPath)

	if _, err := os.Stat(gitDirPath); err == nil {
		err := os.RemoveAll(gitDirPath)
		if err != nil {
			return fmt.Errorf("  [GIT REMOVE ERROR]  %s: %w", gitDirPath, err)
		}
		log.Printf("  [GIT REMOVE SUCCESS]  %s", gitDirPath)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("  [GIT REMOVE ERROR]  %s: %w", gitDirPath, err)
	} else {
		log.Printf("  [GIT REMOVE SKIP]  %s", gitDirPath)
	}

	log.Printf("  [GIT INIT]  %s", repoPath)

	cmd := exec.Command("git", "init")
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("  [GIT INIT ERROR]  %s: %w\n%s", repoPath, err, output)
	}

	log.Printf("  [GIT INIT SUCCESS]  %s", repoPath)
	log.Printf("  [GIT INIT OUTPUT]  %s", output)

	return nil
}

func findModFile(dir string, toChange string) string {
	files, err := os.ReadDir(dir)
	if err != nil {
		log.Fatalf("error reading directory: %v", err)
	}
	for _, file := range files {
		if len(file.Name()) >= 4 && file.Name()[len(file.Name())-4:] == ".mod" {
			log.Printf("  [FILE]  %s\n", file.Name())
			toReplace := getModName(dir, file)
			if toReplace == "" {
				log.Fatalf("module name is not found in %s", file.Name())
			}
			log.Printf("  [TO REPLACE]  %s\n", toReplace)
			changeModFile(dir, file, toChange)
			return toReplace
		}
	}
	return ""
}

func getModName(dir string, file os.DirEntry) string {
	fileText, err := os.ReadFile(dir + "/" + file.Name())
	if err != nil {
		log.Fatalf("error reading file: %v", err)
	}
	
	scanner := bufio.NewScanner(bytes.NewReader(fileText))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "module") {
			parts := strings.Split(line, " ")
			if len(parts) >= 2 {
				return parts[1]
			}
		}
	}
	return ""
}

func checkDir(dir string, toChange string, modName string) {
	files, err := os.ReadDir(dir)
	if err != nil {
		log.Fatalf("error reading directory: %v", err)
	}
	var wg sync.WaitGroup
	log.Printf("  [DIR]  %s\n", dir)
	for _, file := range files {
		if file.IsDir() {
			if file.Name()[0] != '.' {
				log.Printf("  [DIR] %s\n", file.Name())
				subDir := dir + "/" + file.Name()
				if _, err := os.Stat(subDir); err == nil {
					wg.Add(1)
					go func() {
						defer wg.Done()
						checkDir(subDir, toChange, modName)
					}()
				} else {
					log.Printf("  [ERROR] Cannot access directory %s: %v\n", subDir, err)
				}
			}
		} else {
			if len(file.Name()) >= 3 && file.Name()[len(file.Name())-3:] == ".go" {
				log.Printf("  [FILE]  %s\n", file.Name())
				changeGoFile(dir, file, toChange, modName)
			} else {
				log.Printf("  [SKIP]  %s (not a .go file)\n", file.Name())
			}
		}
	}
	wg.Wait()
}

func changeGoFile(dir string, file os.DirEntry, toChange string, modName string) {
	log.Printf("  [MOD NAME]  %s\n", modName)
	log.Printf("  [TO CHANGE]  %s\n", toChange)
	log.Printf("  [FILE CHANGING]  %s\n", file.Name())
	fileText, err := os.ReadFile(dir + "/" + file.Name())
	if err != nil {
		log.Fatalf("error reading file: %v", err)
	}
	
	scanner := bufio.NewScanner(bytes.NewReader(fileText))
	var lines []string
	modified := false
	
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, modName) {
			trimmedLine := strings.TrimSpace(line)
			if strings.HasPrefix(trimmedLine, "import") || strings.HasPrefix(trimmedLine, "\"") {
				if strings.Contains(line, "\""+modName) {
					line = strings.Replace(line, "\""+modName, "\""+toChange, 1)
					modified = true
				} else if strings.Contains(line, modName) {
					line = strings.Replace(line, modName, toChange, 1)
					modified = true
				}
			} else {
				line = strings.Replace(line, modName, toChange, 1)
				modified = true
			}
		}
		lines = append(lines, line)
	}
	
	if modified {
		newContent := strings.Join(lines, "\n")
		err = os.WriteFile(dir+"/"+file.Name(), []byte(newContent), 0644)
		if err != nil {
			log.Fatalf("error writing file: %v", err)
		}
		log.Printf("  [FILE CHANGED]  %s\n", file.Name())
	} else {
		log.Printf("  [FILE UNCHANGED]  %s\n", file.Name())
	}
}

func changeModFile(dir string, file os.DirEntry, toChange string) {
	log.Printf("  [FILE CHANGING]  %s\n", file.Name())
	fileText, err := os.ReadFile(dir + "/" + file.Name())
	if err != nil {
		log.Fatalf("error reading file: %v", err)
	}
	
	scanner := bufio.NewScanner(bytes.NewReader(fileText))
	var lines []string
	
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "module") {
			parts := strings.Split(line, " ")
			if len(parts) >= 2 {
				oldModule := parts[1]
				line = strings.Replace(line, oldModule, toChange, 1)
			}
		}
		lines = append(lines, line)
	}
	
	newContent := strings.Join(lines, "\n")
	err = os.WriteFile(dir+"/"+file.Name(), []byte(newContent), 0644)
	if err != nil {
		log.Fatalf("error writing file: %v", err)
	}
	
	log.Printf("  [FILE CHANGED]  %s\n", file.Name())
}