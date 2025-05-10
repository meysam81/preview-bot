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
	"text/template"
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

type ModeFlag struct {
	value string
}

func (m *ModeFlag) String() string {
	return m.value
}

func (m *ModeFlag) Set(value string) error {
	allowedModes := map[string]bool{
		"comment":    true,
		"delete-all": true,
	}
	if !allowedModes[value] {
		return fmt.Errorf("invalid mode: %s; must be one of: comment, delete-all", value)
	}
	m.value = value
	return nil
}

type FilePath struct {
	value string
}

func (f *FilePath) String() string {
	return f.value
}

func (f *FilePath) Set(value string) error {
	if _, err := os.Stat(value); os.IsNotExist(err) {
		return fmt.Errorf("file does not exist: %s", value)
	}
	f.value = value
	return nil
}

func validateEnvs(requiredEnvs []string) (missingEnvs []string) {
	for _, env := range requiredEnvs {
		if os.Getenv(env) == "" {
			missingEnvs = append(missingEnvs, fmt.Sprintf("%s environment variable is required", env))
		}
	}
	return
}

func main() {
	showHelp := flag.Bool("help", false, "Show help message")
	flag.BoolVar(showHelp, "h", false, "Show help message (shorthand)")

	var mode ModeFlag
	flag.Var(&mode, "m", "Operation mode: comment or delete-all")
	flag.Var(&mode, "mode", "Operation mode: comment or delete-all")

	var filepath FilePath
	flag.Var(&filepath, "f", "Path to the static non-template file")
	flag.Var(&filepath, "file", "Path to the static non-template file (shorthand)")

	var urlIsTemplate bool
	flag.BoolVar(&urlIsTemplate, "url-is-template", false, "URL is a template, e.g., https://pr{{PR_NUMBER}}.example.com")

	flag.Parse()
	args := flag.Args()

	if len(args) < 1 || *showHelp {
		fmt.Println("Usage: go run main.go <repo>")
		fmt.Println()
		fmt.Println("Arguments:")
		fmt.Println("  -m, --mode <mode>  The mode of operation: comment or delete-all.")
		fmt.Println("  -f, --file <path>  Path to the static non-template file. Causes to ignore the template env vars.")
		fmt.Println("  <repo>             The repository name in the format 'owner/repo'.")
		fmt.Println("Environment variables:")
		fmt.Println("  PR_NUMBER          The pull request number to comment on.")
		fmt.Println("  GITHUB_TOKEN       GitHub API token with repo permissions.")
		fmt.Println("  USER_LOGIN         The GitHub user login.")
		fmt.Println("")
		fmt.Println("  DEBUG              Set to 'true' to enable debug output. Default is 'false'.")
		fmt.Println("")
		fmt.Println("  ASSETS_DIR         Where to look for static assets. Default is '/'.")
		fmt.Println("  COMMIT_SHA         The commit SHA.")
		fmt.Println("  TITLE              The comment title. Default is '# Preview Deployment'.")
		fmt.Println("  URL                The deployment URL.")
		os.Exit(0)
	}
	repo := args[0]
	debug := os.Getenv("DEBUG") == "true"

	if mode.String() == "" {
		fmt.Println("No mode specified. Defaulting to 'comment'.")
		mode.Set("comment")
	}

	missingEnvs := []string{""}

	switch mode.String() {
	case "", "comment":
		fmt.Println("Mode set to 'comment'.")
		var requiredEnvs []string
		if filepath.String() == "" {
			requiredEnvs = []string{"PR_NUMBER", "USER_LOGIN", "GITHUB_TOKEN", "URL", "COMMIT_SHA"}
		} else {
			requiredEnvs = []string{"PR_NUMBER", "USER_LOGIN", "GITHUB_TOKEN"}
		}
		missingEnvs = append(missingEnvs, validateEnvs(requiredEnvs)...)
	case "delete-all":
		fmt.Println("Mode set to 'delete-all'.")
		requiredEnvs := []string{"PR_NUMBER", "USER_LOGIN", "GITHUB_TOKEN"}
		missingEnvs = append(missingEnvs, validateEnvs(requiredEnvs)...)
	}

	if len(missingEnvs) > 1 {
		log.Fatalln(strings.Join(missingEnvs, "\n"))
	}

	prNumber := os.Getenv("PR_NUMBER")
	userLogin := os.Getenv("USER_LOGIN")
	githubToken := os.Getenv("GITHUB_TOKEN")

	title := os.Getenv("TITLE")
	if title == "" {
		title = "# Preview Deployment"
	}

	url := os.Getenv("URL")

	if urlIsTemplate {
		urlTmpl, err := template.New("url").Parse(url)
		if err != nil {
			log.Fatalf("Failed to parse URL template: %v", err)
		}
		var buf bytes.Buffer
		err = urlTmpl.Execute(&buf, map[string]string{
			"PR_NUMBER": prNumber,
		})
		if err != nil {
			log.Fatalf("Failed to execute URL template: %v", err)
		}
		url = buf.String()
	}

	commitSha := os.Getenv("COMMIT_SHA")

	assetsDir := os.Getenv("ASSETS_DIR")

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

	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}

	comments, err := getComments(httpClient, repo, prNumber, githubToken)
	if err != nil {
		log.Fatalf("Failed to get comments: %v", err)
	}

	fmt.Println("Deleting all comments...")

	var commentURLs []string
	for _, comment := range comments {
		if comment.User.Login == userLogin {
			commentURLs = append(commentURLs, comment.URL)
		}
	}

	for _, commentURL := range commentURLs {
		if err := deleteComment(httpClient, commentURL, githubToken); err != nil {
			log.Printf("Warning: Failed to delete comment %s: %v", commentURL, err)
		}
	}

	if mode.String() == "delete-all" {
		fmt.Println("Deleted all comments successfully.")
		os.Exit(0)
	}

	fmt.Println("Creating new comment...")

	var commentBody string
	if filepath.String() == "" {
		commentTemplatePath := assetsDir + "/preview-body.md.tpl"
		commentBody, err = processTemplate(commentTemplatePath, map[string]string{
			"TITLE":      title,
			"COMMIT_SHA": commitSha,
			"URL":        url,
		})
		if err != nil {
			log.Fatalf("Failed to process template: %v", err)
		}
	} else {
		fileContent, err := os.ReadFile(filepath.String())
		if err != nil {
			log.Fatalf("Failed to read file: %v", err)
		}
		commentBody = string(fileContent)
	}

	if err := createComment(httpClient, repo, prNumber, commentBody, githubToken); err != nil {
		log.Fatalf("Failed to create comment: %v", err)
	}

	fmt.Println("Comment created successfully.")
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
