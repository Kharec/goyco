package commands

import (
	"errors"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"goyco/internal/config"
	"goyco/internal/database"
	"goyco/internal/repositories"
)

func HandleSeedCommand(cfg *config.Config, name string, args []string) error {
	fs := newFlagSet(name, printSeedUsage)
	if err := parseCommand(fs, args, name); err != nil {
		if errors.Is(err, ErrHelpRequested) {
			return nil
		}
		return err
	}

	return withDatabase(cfg, func(db *gorm.DB) error {
		return db.Transaction(func(tx *gorm.DB) error {
			userRepo := repositories.NewUserRepository(db).WithTx(tx)
			postRepo := repositories.NewPostRepository(db).WithTx(tx)
			voteRepo := repositories.NewVoteRepository(db).WithTx(tx)
			return runSeedCommand(userRepo, postRepo, voteRepo, fs.Args())
		})
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

func clampFlagValue(value *int, min int, name string) {
	if *value < min {
		if !IsJSONOutput() {
			fmt.Fprintf(os.Stderr, "Warning: --%s value %d is too low, clamping to %d\n", name, *value, min)
		}
		*value = min
	}
}

func seedDatabase(userRepo repositories.UserRepository, postRepo repositories.PostRepository, voteRepo repositories.VoteRepository, args []string) error {
	fs := flag.NewFlagSet("seed database", flag.ContinueOnError)
	numPosts := fs.Int("posts", 40, "number of posts to create")
	numUsers := fs.Int("users", 5, "number of additional users to create")
	votesPerPost := fs.Int("votes-per-post", 15, "average votes per post")
	fs.SetOutput(os.Stderr)
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: goyco seed database [--posts <n>] [--users <n>] [--votes-per-post <n>]")
		fmt.Fprintln(os.Stderr, "\nOptions:")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	clampFlagValue(numUsers, 0, "users")
	clampFlagValue(numPosts, 1, "posts")
	clampFlagValue(votesPerPost, 0, "votes-per-post")

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

	seedUser, err := ensureSeedUser(userRepo, string(seedPasswordHash))
	if err != nil {
		return fmt.Errorf("ensure seed user: %w", err)
	}
	if !IsJSONOutput() {
		fmt.Printf("Seed user ready: ID=%d Username=%s\n", seedUser.ID, seedUser.Username)
	}

	generator := newSeedGenerator(string(userPasswordHash))
	allUsers := []database.User{*seedUser}

	users, err := createUsers(generator, userRepo, *numUsers, "Creating users")
	if err != nil {
		return err
	}
	allUsers = append(allUsers, users...)

	posts, err := createPosts(generator, postRepo, seedUser.ID, *numPosts, "Creating posts")
	if err != nil {
		return err
	}

	votes, err := createVotes(generator, voteRepo, allUsers, posts, *votesPerPost, "Creating votes")
	if err != nil {
		return err
	}

	if err := updateScores(generator, postRepo, voteRepo, posts, "Updating scores"); err != nil {
		return err
	}

	if err := lockSeedUser(userRepo, seedUser); err != nil {
		return err
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

func lockSeedUser(userRepo repositories.UserRepository, seedUser *database.User) error {
	if seedUser.Locked {
		return nil
	}

	if err := userRepo.Lock(seedUser.ID); err != nil {
		return fmt.Errorf("lock seed user: %w", err)
	}

	seedUser.Locked = true
	return nil
}

const (
	seedUsername = "seed_admin"
	seedEmail    = "seed_admin@goyco.local"
)

func ensureSeedUser(userRepo repositories.UserRepository, passwordHash string) (*database.User, error) {
	user, err := userRepo.GetByUsername(seedUsername)
	if err == nil {
		return user, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("failed to check if seed user exists: %w", err)
	}

	user = &database.User{
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
		if post.AuthorID == nil {
			return fmt.Errorf("post %d has no author ID", post.ID)
		}
		if _, exists := userIDSet[*post.AuthorID]; !exists {
			return fmt.Errorf("post %d references non-existent author ID %d", post.ID, *post.AuthorID)
		}

		votes, err := voteRepo.GetByPostID(post.ID)
		if err != nil {
			return fmt.Errorf("failed to retrieve votes for post %d: %w", post.ID, err)
		}

		for _, vote := range votes {
			if vote.PostID != post.ID {
				return fmt.Errorf("vote %d references post ID %d but was retrieved for post %d", vote.ID, vote.PostID, post.ID)
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
	}

	return nil
}

type seedGenerator struct {
	passwordHash string
	randSource   *rand.Rand
}

func newSeedGenerator(passwordHash string) *seedGenerator {
	seed := time.Now().UnixNano()
	return &seedGenerator{
		passwordHash: passwordHash,
		randSource:   rand.New(rand.NewSource(seed)),
	}
}

func isRetryableError(err error, keywords ...string) bool {
	if err == nil {
		return false
	}

	errMsg := strings.ToLower(err.Error())
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		for _, keyword := range keywords {
			if strings.Contains(errMsg, keyword) {
				return true
			}
		}
		return false
	}

	var pqErr *pq.Error
	if errors.As(err, &pqErr) && pqErr.Code == "23505" {
		constraintLower := strings.ToLower(pqErr.Constraint)
		errMsgLower := strings.ToLower(pqErr.Message)
		for _, keyword := range keywords {
			if strings.Contains(constraintLower, keyword) || strings.Contains(errMsgLower, keyword) {
				return true
			}
		}
		return false
	}

	if strings.Contains(errMsg, "duplicate") {
		for _, keyword := range keywords {
			if strings.Contains(errMsg, keyword) {
				return true
			}
		}
	}

	return false
}

func createUsers(g *seedGenerator, userRepo repositories.UserRepository, count int, desc string) ([]database.User, error) {
	if count == 0 {
		return nil, nil
	}
	progress := maybeProgress(count, desc)
	users := make([]database.User, 0, count)
	for i := 0; i < count; i++ {
		user, err := g.createSingleUser(userRepo, i+1)
		if err != nil {
			return nil, fmt.Errorf("create random user: %w", err)
		}
		users = append(users, user)
		if progress != nil {
			progress.Increment()
		}
	}
	if progress != nil {
		progress.Complete()
	}
	return users, nil
}

func createPosts(g *seedGenerator, postRepo repositories.PostRepository, authorID uint, count int, desc string) ([]database.Post, error) {
	if count == 0 {
		return nil, nil
	}
	progress := maybeProgress(count, desc)
	posts := make([]database.Post, 0, count)
	for i := 0; i < count; i++ {
		post, err := g.createSinglePost(postRepo, authorID, i+1)
		if err != nil {
			return nil, fmt.Errorf("create random post: %w", err)
		}
		posts = append(posts, post)
		if progress != nil {
			progress.Increment()
		}
	}
	if progress != nil {
		progress.Complete()
	}
	return posts, nil
}

func createVotes(g *seedGenerator, voteRepo repositories.VoteRepository, users []database.User, posts []database.Post, avgVotesPerPost int, desc string) (int, error) {
	if len(posts) == 0 {
		return 0, nil
	}
	progress := maybeProgress(len(posts), desc)
	votes := 0
	for _, post := range posts {
		count, err := g.createVotesForPost(voteRepo, users, post, avgVotesPerPost)
		if err != nil {
			return 0, fmt.Errorf("create random votes for post %d: %w", post.ID, err)
		}
		votes += count
		if progress != nil {
			progress.Increment()
		}
	}
	if progress != nil {
		progress.Complete()
	}
	return votes, nil
}

func updateScores(g *seedGenerator, postRepo repositories.PostRepository, voteRepo repositories.VoteRepository, posts []database.Post, desc string) error {
	if len(posts) == 0 {
		return nil
	}
	progress := maybeProgress(len(posts), desc)
	for _, post := range posts {
		if err := g.updateSinglePostScore(postRepo, voteRepo, post); err != nil {
			return fmt.Errorf("update post scores: %w", err)
		}
		if progress != nil {
			progress.Increment()
		}
	}
	if progress != nil {
		progress.Complete()
	}
	return nil
}

func maybeProgress(count int, desc string) *ProgressIndicator {
	if !IsJSONOutput() && count > 0 {
		return NewProgressIndicator(count, desc)
	}
	return nil
}

func (g *seedGenerator) generateRandomIdentifier() string {
	const length = 12
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	identifier := make([]byte, length)
	for i := range identifier {
		identifier[i] = chars[g.randSource.Intn(len(chars))]
	}
	return string(identifier)
}

func (g *seedGenerator) createSingleUser(userRepo repositories.UserRepository, index int) (database.User, error) {
	const maxRetries = 10
	var lastErr error
	for attempt := range maxRetries {
		randomID := g.generateRandomIdentifier()
		user := &database.User{
			Username:      fmt.Sprintf("user_%s", randomID),
			Email:         fmt.Sprintf("user_%s@goyco.local", randomID),
			Password:      g.passwordHash,
			EmailVerified: true,
		}
		if err := userRepo.Create(user); err != nil {
			lastErr = err
			if !isRetryableError(err, "username", "email", "users_username_key", "users_email_key") {
				return database.User{}, fmt.Errorf("failed to create user (attempt %d/%d): %w", attempt+1, maxRetries, err)
			}
			continue
		}
		return *user, nil
	}
	return database.User{}, fmt.Errorf("failed to create user after %d attempts: %w", maxRetries, lastErr)
}

var (
	sampleTitles  = []string{"Amazing JavaScript Framework", "Python Best Practices", "Go Performance Tips", "Database Optimization", "Web Security Guide", "Machine Learning Basics", "Cloud Architecture", "DevOps Automation", "API Design Patterns", "Frontend Optimization", "Backend Scaling", "Container Orchestration", "Microservices Architecture", "Testing Strategies", "Code Review Process", "Version Control Best Practices", "Continuous Integration", "Monitoring and Alerting", "Error Handling Patterns", "Data Structures Explained"}
	sampleDomains = []string{"example.com", "techblog.org", "devguide.net", "programming.io", "codeexamples.com", "tutorialhub.org", "bestpractices.dev", "learnprogramming.net", "codingtips.org", "softwareengineering.com"}
)

func (g *seedGenerator) createSinglePost(postRepo repositories.PostRepository, authorID uint, index int) (database.Post, error) {
	title := sampleTitles[index%len(sampleTitles)]
	if index >= len(sampleTitles) {
		title = fmt.Sprintf("%s - Part %d", title, (index/len(sampleTitles))+1)
	}
	domain := sampleDomains[index%len(sampleDomains)]
	content := fmt.Sprintf("Autogenerated seed post #%d\n\nThis is sample content for testing purposes. The post discusses %s and provides valuable insights.", index, title)

	const maxRetries = 10
	var lastErr error
	for attempt := range maxRetries {
		randomID := g.generateRandomIdentifier()
		post := &database.Post{
			Title:     title,
			URL:       fmt.Sprintf("https://%s/article/%s", domain, randomID),
			Content:   content,
			AuthorID:  &authorID,
			UpVotes:   0,
			DownVotes: 0,
			Score:     0,
		}
		if err := postRepo.Create(post); err != nil {
			lastErr = err
			if !isRetryableError(err, "url", "posts_url_key") {
				return database.Post{}, fmt.Errorf("failed to create post (attempt %d/%d): %w", attempt+1, maxRetries, err)
			}
			continue
		}
		return *post, nil
	}
	return database.Post{}, fmt.Errorf("failed to create post after %d attempts: %w", maxRetries, lastErr)
}

func (g *seedGenerator) createVotesForPost(voteRepo repositories.VoteRepository, users []database.User, post database.Post, avgVotesPerPost int) (int, error) {
	numVotes := g.randSource.Intn(avgVotesPerPost*2 + 1)

	if numVotes == 0 && avgVotesPerPost > 0 {
		if g.randSource.Intn(5) > 0 {
			numVotes = 1
		}
	}

	totalVotes := 0
	usedUsers := make(map[uint]bool)

	for i := 0; i < numVotes && len(usedUsers) < len(users); i++ {
		userIdx := g.randSource.Intn(len(users))
		user := users[userIdx]

		if usedUsers[user.ID] {
			continue
		}
		usedUsers[user.ID] = true

		voteTypeInt := g.randSource.Intn(10)
		var voteType database.VoteType
		if voteTypeInt < 7 {
			voteType = database.VoteUp
		} else {
			voteType = database.VoteDown
		}

		vote := &database.Vote{
			UserID: &user.ID,
			PostID: post.ID,
			Type:   voteType,
		}

		if err := voteRepo.CreateOrUpdate(vote); err != nil {
			return totalVotes, fmt.Errorf("create or update vote: %w", err)
		}

		totalVotes++
	}

	return totalVotes, nil
}

func (g *seedGenerator) updateSinglePostScore(postRepo repositories.PostRepository, voteRepo repositories.VoteRepository, post database.Post) error {
	upVotes, downVotes, err := voteRepo.GetVoteCountsByPostID(post.ID)
	if err != nil {
		return fmt.Errorf("get vote counts: %w", err)
	}
	post.UpVotes = upVotes
	post.DownVotes = downVotes
	post.Score = upVotes - downVotes
	return postRepo.Update(&post)
}
