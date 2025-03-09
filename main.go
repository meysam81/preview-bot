package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
)

type Comment struct {
	Body string `json:"body"`
	User struct {
		Login string `json:"login"`
	} `json:"user"`
	URL string `json:"url"`
}

type NewComment struct {
	Body string `json:"body"`
}

func main() {
	flag.Parse()
	args := flag.Args()

	if len(args) < 1 {
		fmt.Println("Usage: go run main.go <repo>")
		fmt.Println()
		fmt.Println("Arguments:")
		fmt.Println("  <repo>          The repository name in the format 'owner/repo'.")
		fmt.Println("Environment variables:")
		fmt.Println("  PR_NUMBER       The pull request number to comment on.")
		fmt.Println("  USER_LOGIN      The GitHub user login.")
		fmt.Println("  COMMIT_SHA      The commit SHA.")
		fmt.Println("  URL             The deployment URL.")
		fmt.Println("  TITLE           The comment title. Default is '# Preview Deployment'.")
		fmt.Println("  ASSETS_DIR      Where to look for static assets. Default is '/'.")
		fmt.Println("  DEBUG           Set to 'true' to enable debug output. Default is 'false'.")
		os.Exit(1)
	}

	repo := args[0]

	prNumber := os.Getenv("PR_NUMBER")
	if prNumber == "" {
		log.Fatal("PR_NUMBER environment variable is required")
	}

	userLogin := os.Getenv("USER_LOGIN")
	if userLogin == "" {
		log.Fatal("USER_LOGIN environment variable is required")
	}

	commitSha := os.Getenv("COMMIT_SHA")
	if commitSha == "" {
		log.Fatal("COMMIT_SHA environment variable is required")
	}

	url := os.Getenv("URL")
	if url == "" {
		log.Fatal("URL environment variable is required")
	}

	title := os.Getenv("TITLE")
	if title == "" {
		title = "# Preview Deployment"
	}

	assetsDir := os.Getenv("ASSETS_DIR")
	debug := os.Getenv("DEBUG") == "true"

	if debug {
		log.Println("Debug mode enabled")
		log.Printf("Repository: %s\n", repo)
		log.Printf("PR Number: %s\n", prNumber)
		log.Printf("User Login: %s\n", userLogin)
		log.Printf("Commit SHA: %s\n", commitSha)
		log.Printf("URL: %s\n", url)
		log.Printf("Title: %s\n", title)
		log.Printf("Assets Dir: %s\n", assetsDir)
	}

	commentTemplatePath := assetsDir + "/preview-body.json.tpl"

	comments, err := getComments(repo, prNumber)
	if err != nil {
		log.Fatalf("Failed to get comments: %v", err)
	}

	var commentURLs []string
	for _, comment := range comments {
		if strings.HasPrefix(comment.Body, title) && comment.User.Login == userLogin {
			commentURLs = append(commentURLs, comment.URL)
		}
	}

	commentBody, err := processTemplate(commentTemplatePath, map[string]string{
		"TITLE":      title,
		"COMMIT_SHA": commitSha,
		"URL":        url,
	})
	if err != nil {
		log.Fatalf("Failed to process template: %v", err)
	}

	for _, commentURL := range commentURLs {
		if err := deleteComment(commentURL); err != nil {
			log.Printf("Warning: Failed to delete comment %s: %v", commentURL, err)
		}
	}

	if err := createComment(repo, prNumber, commentBody); err != nil {
		log.Fatalf("Failed to create comment: %v", err)
	}
}

func getComments(repo, prNumber string) ([]Comment, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/issues/%s/comments", repo, prNumber)

	cmd := exec.Command("gh", "api",
		"-HX-GitHub-Api-Version:2022-11-28",
		"-Haccept:application/vnd.github.raw+json",
		url)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("gh api command failed: %v", err)
	}

	if string(output) == "[]" {
		return []Comment{}, nil
	}

	var comments []Comment
	if err := json.Unmarshal(output, &comments); err != nil {
		return nil, fmt.Errorf("failed to unmarshal comments: %v", err)
	}

	return comments, nil
}

func processTemplate(templatePath string, replacements map[string]string) (string, error) {
	content, err := os.ReadFile(templatePath)
	if err != nil {
		return "", fmt.Errorf("failed to read template file: %v", err)
	}

	result := string(content)
	for key, value := range replacements {
		result = strings.ReplaceAll(result, "{{"+key+"}}", value)
	}

	return result, nil
}

func deleteComment(commentURL string) error {
	cmd := exec.Command("gh", "api",
		"-HX-GitHub-Api-Version:2022-11-28",
		"-Haccept:application/vnd.github.raw+json",
		"-XDELETE",
		commentURL)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("gh api delete command failed: %v", err)
	}

	return nil
}

func createComment(repo, prNumber, body string) error {
	url := fmt.Sprintf("https://api.github.com/repos/%s/issues/%s/comments", repo, prNumber)

	newComment := NewComment{Body: body}
	commentData, err := json.Marshal(newComment)
	if err != nil {
		return fmt.Errorf("failed to marshal comment data: %v", err)
	}

	tempFile, err := os.CreateTemp("", "comment-")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	if _, err := tempFile.Write(commentData); err != nil {
		return fmt.Errorf("failed to write to temp file: %v", err)
	}
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %v", err)
	}

	cmd := exec.Command("gh", "api",
		"-HX-GitHub-Api-Version:2022-11-28",
		"-Haccept:application/vnd.github.raw+json",
		"-XPOST",
		url,
		"--input",
		tempFile.Name())

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("gh api post command failed: %v", err)
	}

	return nil
}
