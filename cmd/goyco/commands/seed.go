package commands

import (
	cryptoRand "crypto/rand"
	"errors"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"sync"
	"time"

	"goyco/internal/config"
	"goyco/internal/database"
	"goyco/internal/repositories"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var (
	seedRandSource *rand.Rand
	seedRandOnce   sync.Once
)

func initSeedRand() {
	seedRandOnce.Do(func() {
		seed := time.Now().UnixNano()
		seedBytes := make([]byte, 8)
		if _, err := cryptoRand.Read(seedBytes); err == nil {
			seed = int64(seedBytes[0])<<56 | int64(seedBytes[1])<<48 | int64(seedBytes[2])<<40 | int64(seedBytes[3])<<32 |
				int64(seedBytes[4])<<24 | int64(seedBytes[5])<<16 | int64(seedBytes[6])<<8 | int64(seedBytes[7])
		}
		seedRandSource = rand.New(rand.NewSource(seed))
	})
}

func HandleSeedCommand(cfg *config.Config, name string, args []string) error {
	fs := newFlagSet(name, printSeedUsage)
	if err := parseCommand(fs, args, name); err != nil {
		if errors.Is(err, ErrHelpRequested) {
			return nil
		}
		return err
	}

	return withDatabase(cfg, func(db *gorm.DB) error {
		userRepo := repositories.NewUserRepository(db)
		postRepo := repositories.NewPostRepository(db)
		voteRepo := repositories.NewVoteRepository(db)
		return runSeedCommand(userRepo, postRepo, voteRepo, fs.Args())
	})
}

func runSeedCommand(userRepo repositories.UserRepository, postRepo repositories.PostRepository, voteRepo repositories.VoteRepository, args []string) error {
	if len(args) == 0 {
		printSeedUsage()
		return errors.New("missing seed subcommand")
	}

	switch args[0] {
	case "database":
		return seedDatabase(userRepo, postRepo, voteRepo, args[1:])
	case "help", "-h", "--help":
		printSeedUsage()
		return nil
	default:
		printSeedUsage()
		return fmt.Errorf("unknown seed subcommand: %s", args[0])
	}
}

func printSeedUsage() {
	fmt.Fprintln(os.Stderr, "Seed subcommands:")
	fmt.Fprintln(os.Stderr, "  database [--posts <n>] [--users <n>] [--votes-per-post <n>]")
	fmt.Fprintln(os.Stderr, "    --posts: number of posts to create (default: 40)")
	fmt.Fprintln(os.Stderr, "    --users: number of additional users to create (default: 5)")
	fmt.Fprintln(os.Stderr, "    --votes-per-post: average votes per post (default: 15)")
}

func seedDatabase(userRepo repositories.UserRepository, postRepo repositories.PostRepository, voteRepo repositories.VoteRepository, args []string) error {
	fs := flag.NewFlagSet("seed database", flag.ContinueOnError)
	numPosts := fs.Int("posts", 40, "number of posts to create")
	numUsers := fs.Int("users", 5, "number of additional users to create")
	votesPerPost := fs.Int("votes-per-post", 15, "average votes per post")
	fs.SetOutput(os.Stderr)

	if err := fs.Parse(args); err != nil {
		return err
	}

	originalUsers := *numUsers
	originalPosts := *numPosts
	originalVotesPerPost := *votesPerPost

	if *numUsers < 0 {
		if !IsJSONOutput() {
			fmt.Fprintf(os.Stderr, "Warning: --users value %d is negative, clamping to 0\n", *numUsers)
		}
		*numUsers = 0
	}

	if *numPosts <= 0 {
		if !IsJSONOutput() {
			fmt.Fprintf(os.Stderr, "Warning: --posts value %d is too low, clamping to 1\n", *numPosts)
		}
		*numPosts = 1
	}

	if *votesPerPost < 0 {
		if !IsJSONOutput() {
			fmt.Fprintf(os.Stderr, "Warning: --votes-per-post value %d is negative, clamping to 0\n", *votesPerPost)
		}
		*votesPerPost = 0
	}

	if !IsJSONOutput() && (originalUsers != *numUsers || originalPosts != *numPosts || originalVotesPerPost != *votesPerPost) {
		fmt.Fprintf(os.Stderr, "Using clamped values: --users=%d --posts=%d --votes-per-post=%d\n", *numUsers, *numPosts, *votesPerPost)
	}

	if !IsJSONOutput() {
		fmt.Println("Starting database seeding...")
	}

	seedPassword := "seed-password"
	userPassword := "password123"

	seedPasswordHash, err := bcrypt.GenerateFromPassword([]byte(seedPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("precompute seed password hash: %w", err)
	}

	userPasswordHash, err := bcrypt.GenerateFromPassword([]byte(userPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("precompute user password hash: %w", err)
	}

	spinner := NewSpinner("Creating seed user")
	if !IsJSONOutput() {
		spinner.Spin()
	}

	seedUser, err := ensureSeedUser(userRepo, string(seedPasswordHash))
	if err != nil {
		if !IsJSONOutput() {
			spinner.Complete()
		}
		return fmt.Errorf("ensure seed user: %w", err)
	}
	if !IsJSONOutput() {
		spinner.Complete()
		fmt.Printf("Seed user ready: ID=%d Username=%s\n", seedUser.ID, seedUser.Username)
	}

	processor := NewParallelProcessor()
	processor.SetPasswordHash(string(userPasswordHash))

	var progress *ProgressIndicator
	if !IsJSONOutput() && *numUsers > 0 {
		progress = NewProgressIndicator(*numUsers, "Creating users (parallel)")
	}
	users, err := processor.CreateUsersInParallel(userRepo, *numUsers, progress)
	if err != nil {
		return fmt.Errorf("create random users: %w", err)
	}
	if !IsJSONOutput() && progress != nil {
		progress.Complete()
	}

	allUsers := append([]database.User{*seedUser}, users...)

	if !IsJSONOutput() && *numPosts > 0 {
		progress = NewProgressIndicator(*numPosts, "Creating posts (parallel)")
	}
	posts, err := processor.CreatePostsInParallel(postRepo, seedUser.ID, *numPosts, progress)
	if err != nil {
		return fmt.Errorf("create random posts: %w", err)
	}
	if !IsJSONOutput() && progress != nil {
		progress.Complete()
	}

	if !IsJSONOutput() && len(posts) > 0 {
		progress = NewProgressIndicator(len(posts), "Creating votes (parallel)")
	}
	votes, err := processor.CreateVotesInParallel(voteRepo, allUsers, posts, *votesPerPost, progress)
	if err != nil {
		return fmt.Errorf("create random votes: %w", err)
	}
	if !IsJSONOutput() && progress != nil {
		progress.Complete()
	}

	if !IsJSONOutput() && len(posts) > 0 {
		progress = NewProgressIndicator(len(posts), "Updating scores (parallel)")
	}
	err = processor.UpdatePostScoresInParallel(postRepo, voteRepo, posts, progress)
	if err != nil {
		return fmt.Errorf("update post scores: %w", err)
	}
	if !IsJSONOutput() && progress != nil {
		progress.Complete()
	}

	if err := validateSeedConsistency(voteRepo, allUsers, posts); err != nil {
		return fmt.Errorf("seed consistency validation failed: %w", err)
	}

	if IsJSONOutput() {
		outputJSON(map[string]any{
			"action": "seed_completed",
			"users":  len(allUsers),
			"posts":  len(posts),
			"votes":  votes,
			"seed_user": map[string]any{
				"id":       seedUser.ID,
				"username": seedUser.Username,
			},
		})
	} else {
		fmt.Println("Database seeding completed successfully!")
		fmt.Printf("Created %d users, %d posts, and %d votes\n", len(allUsers), len(posts), votes)
	}

	return nil
}

const (
	seedUsername = "seed_admin"
	seedEmail    = "seed_admin@goyco.local"
)

func ensureSeedUser(userRepo repositories.UserRepository, passwordHash string) (*database.User, error) {
	if user, err := userRepo.GetByUsername(seedUsername); err == nil {
		return user, nil
	}

	user := &database.User{
		Username:      seedUsername,
		Email:         seedEmail,
		Password:      passwordHash,
		EmailVerified: true,
	}

	if err := userRepo.Create(user); err != nil {
		return nil, fmt.Errorf("failed to create seed user: %w", err)
	}

	return user, nil
}

func getVoteCounts(voteRepo repositories.VoteRepository, postID uint) (int, int, error) {
	return voteRepo.GetVoteCountsByPostID(postID)
}

func validateSeedConsistency(voteRepo repositories.VoteRepository, users []database.User, posts []database.Post) error {
	userIDSet := make(map[uint]struct{}, len(users))
	for _, user := range users {
		userIDSet[user.ID] = struct{}{}
	}

	postIDSet := make(map[uint]struct{}, len(posts))
	for _, post := range posts {
		postIDSet[post.ID] = struct{}{}
	}

	for _, post := range posts {
		if err := validatePost(post, userIDSet); err != nil {
			return err
		}

		votes, err := voteRepo.GetByPostID(post.ID)
		if err != nil {
			return fmt.Errorf("failed to retrieve votes for post %d: %w", post.ID, err)
		}

		if err := validateVotesForPost(post.ID, votes, userIDSet, postIDSet); err != nil {
			return err
		}
	}

	return nil
}

func validatePost(post database.Post, userIDSet map[uint]struct{}) error {
	if post.AuthorID == nil {
		return fmt.Errorf("post %d has no author ID", post.ID)
	}

	if _, exists := userIDSet[*post.AuthorID]; !exists {
		return fmt.Errorf("post %d references non-existent author ID %d", post.ID, *post.AuthorID)
	}

	return nil
}

func validateVotesForPost(postID uint, votes []database.Vote, userIDSet map[uint]struct{}, postIDSet map[uint]struct{}) error {
	for _, vote := range votes {
		if vote.PostID != postID {
			return fmt.Errorf("vote %d references post ID %d but was retrieved for post %d", vote.ID, vote.PostID, postID)
		}

		if _, exists := postIDSet[vote.PostID]; !exists {
			return fmt.Errorf("vote %d references non-existent post ID %d", vote.ID, vote.PostID)
		}

		if vote.UserID != nil {
			if _, exists := userIDSet[*vote.UserID]; !exists {
				return fmt.Errorf("vote %d references non-existent user ID %d", vote.ID, *vote.UserID)
			}
		}

		if vote.Type != database.VoteUp && vote.Type != database.VoteDown {
			return fmt.Errorf("vote %d has invalid type %q", vote.ID, vote.Type)
		}
	}

	return nil
}
