package commands

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"gorm.io/gorm"
	"goyco/internal/config"
	"goyco/internal/repositories"
)

func HandlePruneCommand(cfg *config.Config, name string, args []string) error {
	fs := newFlagSet(name, printPruneUsage)
	if err := parseCommand(fs, args, name); err != nil {
		if errors.Is(err, ErrHelpRequested) {
			return nil
		}
		return err
	}

	return withDatabase(cfg, func(db *gorm.DB) error {
		userRepo := repositories.NewUserRepository(db)
		postRepo := repositories.NewPostRepository(db)
		return runPruneCommand(cfg, userRepo, postRepo, fs.Args())
	})
}

func runPruneCommand(_ *config.Config, userRepo repositories.UserRepository, postRepo repositories.PostRepository, args []string) error {
	if len(args) == 0 {
		printPruneUsage()
		return errors.New("missing prune subcommand")
	}

	switch args[0] {
	case "posts":
		return prunePosts(postRepo, args[1:])
	case "users":
		return pruneUsers(userRepo, postRepo, args[1:])
	case "all":
		return pruneAll(userRepo, postRepo, args[1:])
	case "help", "-h", "--help":
		printPruneUsage()
		return nil
	default:
		printPruneUsage()
		return fmt.Errorf("unknown prune subcommand: %s", args[0])
	}
}

func printPruneUsage() {
	fmt.Fprintln(os.Stderr, "Prune subcommands:")
	fmt.Fprintln(os.Stderr, "  posts     hard delete posts of deleted users")
	fmt.Fprintln(os.Stderr, "  users     hard delete all users [--with-posts]")
	fmt.Fprintln(os.Stderr, "  all       hard delete all users and posts")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "WARNING: These operations are irreversible!")
	fmt.Fprintln(os.Stderr, "Use --dry-run to preview what would be deleted without actually deleting.")
}

func prunePosts(postRepo repositories.PostRepository, args []string) error {
	fs := flag.NewFlagSet("prune posts", flag.ContinueOnError)
	dryRun := fs.Bool("dry-run", false, "preview what would be deleted without actually deleting")
	fs.SetOutput(os.Stderr)

	if err := fs.Parse(args); err != nil {
		return err
	}

	posts, err := postRepo.GetPostsByDeletedUsers()
	if err != nil {
		return fmt.Errorf("get posts by deleted users: %w", err)
	}

	if len(posts) == 0 {
		fmt.Println("No posts found for deleted users")
		return nil
	}

	fmt.Printf("Found %d posts by deleted users:\n", len(posts))
	for _, post := range posts {
		authorName := "(deleted)"
		if post.Author.ID != 0 {
			authorName = post.Author.Username
		}
		fmt.Printf("  ID=%d Title=%s Author=%s URL=%s\n",
			post.ID, post.Title, authorName, post.URL)
	}

	if *dryRun {
		fmt.Println("\nDry run: No posts were actually deleted")
		return nil
	}

	fmt.Printf("\nAre you sure you want to permanently delete %d posts? (yes/no): ", len(posts))
	var confirmation string
	if _, err := fmt.Scanln(&confirmation); err != nil {
		return fmt.Errorf("read confirmation: %w", err)
	}

	if confirmation != "yes" {
		fmt.Println("Operation cancelled")
		return nil
	}

	deletedCount, err := postRepo.HardDeletePostsByDeletedUsers()
	if err != nil {
		return fmt.Errorf("hard delete posts: %w", err)
	}

	fmt.Printf("Successfully deleted %d posts\n", deletedCount)
	return nil
}

func pruneUsers(userRepo repositories.UserRepository, postRepo repositories.PostRepository, args []string) error {
	fs := flag.NewFlagSet("prune users", flag.ContinueOnError)
	dryRun := fs.Bool("dry-run", false, "preview what would be deleted without actually deleting")
	deletePosts := fs.Bool("with-posts", false, "also delete all posts when deleting users")
	fs.SetOutput(os.Stderr)

	if err := fs.Parse(args); err != nil {
		return err
	}

	users, err := userRepo.GetAll(0, 0)
	if err != nil {
		return fmt.Errorf("get users: %w", err)
	}

	userCount := len(users)
	if userCount == 0 {
		fmt.Println("No users found to delete")
		return nil
	}

	var postCount int64 = 0
	if *deletePosts {
		postCount, err = postRepo.Count()
		if err != nil {
			return fmt.Errorf("get post count: %w", err)
		}
	}

	fmt.Printf("Found %d users", userCount)
	if *deletePosts {
		fmt.Printf(" and %d posts", postCount)
	}
	fmt.Println(" to delete")

	fmt.Println("\nUsers to be deleted:")
	for _, user := range users {
		fmt.Printf("  ID=%d Username=%s Email=%s\n", user.ID, user.Username, user.Email)
	}

	if *dryRun {
		fmt.Println("\nDry run: No data was actually deleted")
		return nil
	}

	confirmMsg := fmt.Sprintf("\nAre you sure you want to permanently delete %d users", userCount)
	if *deletePosts {
		confirmMsg += fmt.Sprintf(" and %d posts", postCount)
	}
	confirmMsg += "? (yes/no): "
	fmt.Print(confirmMsg)

	var confirmation string
	if _, err := fmt.Scanln(&confirmation); err != nil {
		return fmt.Errorf("read confirmation: %w", err)
	}

	if confirmation != "yes" {
		fmt.Println("Operation cancelled")
		return nil
	}

	if *deletePosts {
		totalDeleted, err := userRepo.HardDeleteAll()
		if err != nil {
			return fmt.Errorf("hard delete all users and posts: %w", err)
		}
		fmt.Printf("Successfully deleted %d total records (users, posts, votes, etc.)\n", totalDeleted)
	} else {
		deletedCount := 0
		for _, user := range users {
			if err := userRepo.SoftDeleteWithPosts(user.ID); err != nil {
				return fmt.Errorf("soft delete user %d: %w", user.ID, err)
			}
			deletedCount++
		}
		fmt.Printf("Successfully soft deleted %d users (posts preserved)\n", deletedCount)
	}

	return nil
}

func pruneAll(userRepo repositories.UserRepository, postRepo repositories.PostRepository, args []string) error {
	fs := flag.NewFlagSet("prune all", flag.ContinueOnError)
	dryRun := fs.Bool("dry-run", false, "preview what would be deleted without actually deleting")
	fs.SetOutput(os.Stderr)

	if err := fs.Parse(args); err != nil {
		return err
	}

	userCount, err := userRepo.GetAll(0, 0)
	if err != nil {
		return fmt.Errorf("get user count: %w", err)
	}

	postCount, err := postRepo.Count()
	if err != nil {
		return fmt.Errorf("get post count: %w", err)
	}

	fmt.Printf("Found %d users and %d posts to delete\n", len(userCount), postCount)

	if *dryRun {
		fmt.Println("\nDry run: No data was actually deleted")
		return nil
	}

	fmt.Printf("\nAre you sure you want to permanently delete ALL %d users and %d posts? (yes/no): ", len(userCount), postCount)
	var confirmation string
	if _, err := fmt.Scanln(&confirmation); err != nil {
		return fmt.Errorf("read confirmation: %w", err)
	}

	if confirmation != "yes" {
		fmt.Println("Operation cancelled")
		return nil
	}

	totalDeleted, err := userRepo.HardDeleteAll()
	if err != nil {
		return fmt.Errorf("hard delete all: %w", err)
	}

	fmt.Printf("Successfully deleted %d total records (users, posts, votes, etc.)\n", totalDeleted)
	return nil
}
