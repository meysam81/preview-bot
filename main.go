package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
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
		fmt.Println("  GITHUB_TOKEN    GitHub API token with repo permissions.")
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

	githubToken := os.Getenv("GITHUB_TOKEN")
	if githubToken == "" {
		log.Fatal("GITHUB_TOKEN environment variable is required")
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

	commentTemplatePath := assetsDir + "/preview-body.md.tpl"

	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}

	comments, err := getComments(httpClient, repo, prNumber, githubToken)
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
		if err := deleteComment(httpClient, commentURL, githubToken); err != nil {
			log.Printf("Warning: Failed to delete comment %s: %v", commentURL, err)
		}
	}

	if err := createComment(httpClient, repo, prNumber, commentBody, githubToken); err != nil {
		log.Fatalf("Failed to create comment: %v", err)
	}
}

func getComments(client *http.Client, repo, prNumber, token string) ([]Comment, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/issues/%s/comments", repo, prNumber)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Accept", "application/vnd.github.raw+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, body)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	if string(body) == "[]" {
		return []Comment{}, nil
	}

	var comments []Comment
	if err := json.Unmarshal(body, &comments); err != nil {
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

func deleteComment(client *http.Client, commentURL, token string) error {
	req, err := http.NewRequest("DELETE", commentURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Accept", "application/vnd.github.raw+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, body)
	}

	return nil
}

func createComment(client *http.Client, repo, prNumber, body, token string) error {
	url := fmt.Sprintf("https://api.github.com/repos/%s/issues/%s/comments", repo, prNumber)

	newComment := NewComment{Body: body}
	commentData, err := json.Marshal(newComment)
	if err != nil {
		return fmt.Errorf("failed to marshal comment data: %v", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(commentData))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Accept", "application/vnd.github.raw+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, body)
	}

	return nil
}
