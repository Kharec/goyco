package commands

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"

	"goyco/internal/config"
	"goyco/internal/database"
	"goyco/internal/repositories"
	"goyco/internal/security"
	"goyco/internal/services"

	"gorm.io/gorm"
)

func HandlePostCommand(cfg *config.Config, name string, args []string) error {
	fs := newFlagSet(name, printPostUsage)
	if err := parseCommand(fs, args, name); err != nil {
		if errors.Is(err, ErrHelpRequested) {
			return nil
		}
		return err
	}

	return withDatabase(cfg, func(db *gorm.DB) error {
		repo := repositories.NewPostRepository(db)
		voteRepo := repositories.NewVoteRepository(db)
		voteService := services.NewVoteService(voteRepo, repo, db)
		postQueries := services.NewPostQueries(repo, voteService)
		return runPostCommand(postQueries, repo, fs.Args())
	})
}

func runPostCommand(postQueries *services.PostQueries, repo repositories.PostRepository, args []string) error {
	if len(args) == 0 {
		printPostUsage()
		return errors.New("missing post subcommand")
	}

	switch args[0] {
	case "delete":
		return postDelete(repo, args[1:])
	case "list":
		return postList(postQueries, args[1:])
	case "search":
		return postSearch(postQueries, args[1:])
	case "help", "-h", "--help":
		printPostUsage()
		return nil
	default:
		printPostUsage()
		return fmt.Errorf("unknown post subcommand: %s", args[0])
	}
}

func printPostUsage() {
	fmt.Fprintln(os.Stderr, "Post subcommands:")
	fmt.Fprintln(os.Stderr, "  delete <id>")
	fmt.Fprintln(os.Stderr, "  list [--limit <n>] [--offset <n>] [--user-id <id>]")
	fmt.Fprintln(os.Stderr, "  search <term> [--limit <n>] [--offset <n>]")
}

func postDelete(repo repositories.PostRepository, args []string) error {
	fs := flag.NewFlagSet("post delete", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() == 0 {
		fs.Usage()
		return errors.New("post ID is required")
	}

	idStr := fs.Arg(0)
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		return fmt.Errorf("invalid post ID: %s", idStr)
	}

	if id == 0 {
		return errors.New("post ID must be greater than 0")
	}

	if err := repo.Delete(uint(id)); err != nil {
		return fmt.Errorf("delete post: %w", err)
	}

	if IsJSONOutput() {
		outputJSON(map[string]interface{}{
			"action": "post_deleted",
			"id":     id,
		})
	} else {
		fmt.Printf("Post deleted: ID=%d\n", id)
	}
	return nil
}

func postList(postQueries *services.PostQueries, args []string) error {
	fs := flag.NewFlagSet("post list", flag.ContinueOnError)
	limit := fs.Int("limit", 0, "max number of posts to list")
	offset := fs.Int("offset", 0, "number of posts to skip")
	userID := fs.Uint("user-id", 0, "filter posts by author id")
	fs.SetOutput(os.Stderr)

	if err := fs.Parse(args); err != nil {
		return err
	}

	opts := services.QueryOptions{
		Limit:  *limit,
		Offset: *offset,
	}

	ctx := services.VoteContext{}

	var (
		posts []database.Post
		err   error
	)

	if *userID > 0 {
		posts, err = postQueries.GetByUserID(*userID, opts, ctx)
	} else {
		posts, err = postQueries.GetAll(opts, ctx)
	}
	if err != nil {
		return fmt.Errorf("list posts: %w", err)
	}

	if IsJSONOutput() {
		type postJSON struct {
			ID        uint   `json:"id"`
			Title     string `json:"title"`
			AuthorID  uint   `json:"author_id"`
			Score     int    `json:"score"`
			CreatedAt string `json:"created_at"`
		}
		postsJSON := make([]postJSON, len(posts))
		for i, p := range posts {
			authorID := uint(0)
			if p.AuthorID != nil {
				authorID = *p.AuthorID
			}
			if p.Author.ID != 0 {
				authorID = p.Author.ID
			}
			postsJSON[i] = postJSON{
				ID:        p.ID,
				Title:     p.Title,
				AuthorID:  authorID,
				Score:     p.Score,
				CreatedAt: p.CreatedAt.Format("2006-01-02 15:04:05"),
			}
		}
		outputJSON(map[string]interface{}{
			"posts": postsJSON,
			"count": len(postsJSON),
		})
		return nil
	}

	if len(posts) == 0 {
		fmt.Println("No posts found")
		return nil
	}

	maxIDWidth := 2
	maxTitleWidth := 5
	maxAuthorIDWidth := 8
	maxScoreWidth := 5
	maxCreatedAtWidth := 10

	for _, p := range posts {
		authorID := uint(0)
		if p.AuthorID != nil {
			authorID = *p.AuthorID
		}
		if p.Author.ID != 0 {
			authorID = p.Author.ID
		}
		truncatedTitle := truncate(p.Title, 40)
		createdAtStr := p.CreatedAt.Format("2006-01-02 15:04:05")

		if len(fmt.Sprintf("%d", p.ID)) > maxIDWidth {
			maxIDWidth = len(fmt.Sprintf("%d", p.ID))
		}
		if len(truncatedTitle) > maxTitleWidth {
			maxTitleWidth = len(truncatedTitle)
		}
		if len(fmt.Sprintf("%d", authorID)) > maxAuthorIDWidth {
			maxAuthorIDWidth = len(fmt.Sprintf("%d", authorID))
		}
		if len(fmt.Sprintf("%d", p.Score)) > maxScoreWidth {
			maxScoreWidth = len(fmt.Sprintf("%d", p.Score))
		}
		if len(createdAtStr) > maxCreatedAtWidth {
			maxCreatedAtWidth = len(createdAtStr)
		}
	}

	fmt.Printf("%-*s %-*s %-*s %-*s %s\n",
		maxIDWidth, "ID",
		maxTitleWidth, "Title",
		maxAuthorIDWidth, "AuthorID",
		maxScoreWidth, "Score",
		"CreatedAt")

	for _, p := range posts {
		authorID := uint(0)
		if p.AuthorID != nil {
			authorID = *p.AuthorID
		}
		if p.Author.ID != 0 {
			authorID = p.Author.ID
		}
		truncatedTitle := truncate(p.Title, 40)
		createdAtStr := p.CreatedAt.Format("2006-01-02 15:04:05")

		fmt.Printf("%-*d %-*s %-*d %-*d %s\n",
			maxIDWidth, p.ID,
			maxTitleWidth, truncatedTitle,
			maxAuthorIDWidth, authorID,
			maxScoreWidth, p.Score,
			createdAtStr)
	}
	return nil
}

func postSearch(postQueries *services.PostQueries, args []string) error {
	fs := flag.NewFlagSet("post search", flag.ContinueOnError)
	limit := fs.Int("limit", 10, "max number of posts to return")
	offset := fs.Int("offset", 0, "number of posts to skip")
	fs.SetOutput(os.Stderr)

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() == 0 {
		fs.Usage()
		return errors.New("search term is required")
	}

	if *limit < 0 {
		return errors.New("limit must be non-negative")
	}
	if *offset < 0 {
		return errors.New("offset must be non-negative")
	}

	sanitizer := security.NewInputSanitizer()
	term := fs.Arg(0)
	sanitizedTerm, err := sanitizer.SanitizeSearchTerm(term)
	if err != nil {
		return fmt.Errorf("search term validation: %w", err)
	}

	opts := services.QueryOptions{
		Limit:  *limit,
		Offset: *offset,
	}

	ctx := services.VoteContext{}

	posts, err := postQueries.GetSearch(sanitizedTerm, opts, ctx)
	if err != nil {
		return fmt.Errorf("search posts: %w", err)
	}

	if IsJSONOutput() {
		type postJSON struct {
			ID        uint   `json:"id"`
			Title     string `json:"title"`
			AuthorID  uint   `json:"author_id"`
			Score     int    `json:"score"`
			CreatedAt string `json:"created_at"`
		}
		postsJSON := make([]postJSON, len(posts))
		for i, p := range posts {
			authorID := uint(0)
			if p.AuthorID != nil {
				authorID = *p.AuthorID
			}
			if p.Author.ID != 0 {
				authorID = p.Author.ID
			}
			postsJSON[i] = postJSON{
				ID:        p.ID,
				Title:     p.Title,
				AuthorID:  authorID,
				Score:     p.Score,
				CreatedAt: p.CreatedAt.Format("2006-01-02 15:04:05"),
			}
		}
		outputJSON(map[string]interface{}{
			"search_term": sanitizedTerm,
			"posts":       postsJSON,
			"count":       len(postsJSON),
		})
		return nil
	}

	if len(posts) == 0 {
		fmt.Println("No posts found matching your search")
		return nil
	}

	fmt.Printf("%-4s %-40s %-12s %-6s %-19s\n", "ID", "Title", "AuthorID", "Score", "CreatedAt")
	for _, p := range posts {
		authorID := uint(0)
		if p.AuthorID != nil {
			authorID = *p.AuthorID
		}
		if p.Author.ID != 0 {
			authorID = p.Author.ID
		}
		fmt.Printf("%-4d %-40s %-12d %-6d %-19s\n", p.ID, truncate(p.Title, 40), authorID, p.Score, p.CreatedAt.Format("2006-01-02 15:04:05"))
	}
	return nil
}
