package commands

import (
	"crypto/rand"
	"errors"
	"flag"
	"fmt"
	"math/big"
	"os"

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

	if *numUsers < 0 {
		return fmt.Errorf("invalid value for --users: %d (must be >= 0)", *numUsers)
	}
	if *numPosts <= 0 {
		return fmt.Errorf("invalid value for --posts: %d (must be > 0)", *numPosts)
	}
	if *votesPerPost < 0 {
		return fmt.Errorf("invalid value for --votes-per-post: %d (must be >= 0)", *votesPerPost)
	}

	if !IsJSONOutput() {
		fmt.Println("Starting database seeding...")
	}

	spinner := NewSpinner("Creating seed user")
	if !IsJSONOutput() {
		spinner.Spin()
	}

	seedUser, err := ensureSeedUser(userRepo)
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

	if IsJSONOutput() {
		outputJSON(map[string]interface{}{
			"action": "seed_completed",
			"users":  len(allUsers),
			"posts":  len(posts),
			"votes":  votes,
			"seed_user": map[string]interface{}{
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

func ensureSeedUser(userRepo repositories.UserRepository) (*database.User, error) {
	seedUsername := "seed_admin"
	seedEmail := "seed_admin@goyco.local"
	seedPassword := "seed-password"

	user, err := userRepo.GetByEmail(seedEmail)
	if err == nil {
		return user, nil
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(seedPassword), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	user = &database.User{
		Username:      seedUsername,
		Email:         seedEmail,
		Password:      string(hashedPassword),
		EmailVerified: true,
	}

	if err := userRepo.Create(user); err != nil {
		return nil, fmt.Errorf("create seed user: %w", err)
	}

	return user, nil
}

func createRandomUsers(userRepo repositories.UserRepository, count int) ([]database.User, error) {
	var users []database.User

	for i := range count {
		username := fmt.Sprintf("user_%d", i+1)
		email := fmt.Sprintf("user_%d@goyco.local", i+1)
		password := "password123"

		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return nil, fmt.Errorf("hash password for user %d: %w", i+1, err)
		}

		user := &database.User{
			Username:      username,
			Email:         email,
			Password:      string(hashedPassword),
			EmailVerified: true,
		}

		if err := userRepo.Create(user); err != nil {
			return nil, fmt.Errorf("create user %d: %w", i+1, err)
		}

		users = append(users, *user)
	}

	return users, nil
}

func createRandomPosts(postRepo repositories.PostRepository, authorID uint, count int) ([]database.Post, error) {
	var posts []database.Post

	sampleTitles := []string{
		"Amazing JavaScript Framework",
		"Python Best Practices",
		"Go Performance Tips",
		"Database Optimization",
		"Web Security Guide",
		"Machine Learning Basics",
		"Cloud Architecture",
		"DevOps Automation",
		"API Design Patterns",
		"Frontend Optimization",
		"Backend Scaling",
		"Container Orchestration",
		"Microservices Architecture",
		"Testing Strategies",
		"Code Review Process",
		"Version Control Best Practices",
		"Continuous Integration",
		"Monitoring and Alerting",
		"Error Handling Patterns",
		"Data Structures Explained",
	}

	sampleDomains := []string{
		"example.com",
		"techblog.org",
		"devguide.net",
		"programming.io",
		"codeexamples.com",
		"tutorialhub.org",
		"bestpractices.dev",
		"learnprogramming.net",
		"codingtips.org",
		"softwareengineering.com",
	}

	for i := range count {
		title := sampleTitles[i%len(sampleTitles)]
		if i >= len(sampleTitles) {
			title = fmt.Sprintf("%s - Part %d", title, (i/len(sampleTitles))+1)
		}

		domain := sampleDomains[i%len(sampleDomains)]
		path := generateRandomPath()
		url := fmt.Sprintf("https://%s%s", domain, path)

		content := fmt.Sprintf("Autogenerated seed post #%d\n\nThis is sample content for testing purposes. The post discusses %s and provides valuable insights.", i+1, title)

		post := &database.Post{
			Title:     title,
			URL:       url,
			Content:   content,
			AuthorID:  &authorID,
			UpVotes:   0,
			DownVotes: 0,
			Score:     0,
		}

		if err := postRepo.Create(post); err != nil {
			return nil, fmt.Errorf("create post %d: %w", i+1, err)
		}

		posts = append(posts, *post)
	}

	return posts, nil
}

func generateRandomPath() string {
	pathLength, _ := rand.Int(rand.Reader, big.NewInt(20))
	path := "/article/"

	for i := int64(0); i < pathLength.Int64()+5; i++ {
		randomChar, _ := rand.Int(rand.Reader, big.NewInt(26))
		path += string(rune('a' + randomChar.Int64()))
	}

	return path
}

func createRandomVotes(voteRepo repositories.VoteRepository, users []database.User, posts []database.Post, avgVotesPerPost int) (int, error) {
	totalVotes := 0

	for _, post := range posts {
		voteCount, _ := rand.Int(rand.Reader, big.NewInt(int64(avgVotesPerPost*2)+1))
		numVotes := int(voteCount.Int64())

		if numVotes == 0 && avgVotesPerPost > 0 {
			chance, _ := rand.Int(rand.Reader, big.NewInt(5))
			if chance.Int64() > 0 {
				numVotes = 1
			}
		}

		usedUsers := make(map[uint]bool)
		for i := 0; i < numVotes && len(usedUsers) < len(users); i++ {
			userIdx, _ := rand.Int(rand.Reader, big.NewInt(int64(len(users))))
			user := users[userIdx.Int64()]

			if usedUsers[user.ID] {
				continue
			}
			usedUsers[user.ID] = true

			voteTypeInt, _ := rand.Int(rand.Reader, big.NewInt(10))
			var voteType database.VoteType
			if voteTypeInt.Int64() < 7 {
				voteType = database.VoteUp
			} else {
				voteType = database.VoteDown
			}

			vote := &database.Vote{
				UserID: &user.ID,
				PostID: post.ID,
				Type:   voteType,
			}

			if err := voteRepo.Create(vote); err != nil {
				return totalVotes, fmt.Errorf("create vote for post %d: %w", post.ID, err)
			}

			totalVotes++
		}
	}

	return totalVotes, nil
}

func updatePostScores(postRepo repositories.PostRepository, voteRepo repositories.VoteRepository, posts []database.Post) error {
	for _, post := range posts {
		upVotes, downVotes, err := getVoteCounts(voteRepo, post.ID)
		if err != nil {
			return fmt.Errorf("get vote counts for post %d: %w", post.ID, err)
		}

		post.UpVotes = upVotes
		post.DownVotes = downVotes
		post.Score = upVotes - downVotes

		if err := postRepo.Update(&post); err != nil {
			return fmt.Errorf("update post %d scores: %w", post.ID, err)
		}
	}

	return nil
}

func getVoteCounts(voteRepo repositories.VoteRepository, postID uint) (int, int, error) {
	votes, err := voteRepo.GetByPostID(postID)
	if err != nil {
		return 0, 0, err
	}

	upVotes := 0
	downVotes := 0

	for _, vote := range votes {
		switch vote.Type {
		case database.VoteUp:
			upVotes++
		case database.VoteDown:
			downVotes++
		}
	}

	return upVotes, downVotes, nil
}
