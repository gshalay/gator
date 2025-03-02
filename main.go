package main

import (
	"context"
	"database/sql"
	"encoding/xml"
	"fmt"
	jsonconfig "gator/internal/config"
	"gator/internal/database"
	"html"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

const (
	MIN_ARGS = 2
	FEED_URL = "https://www.wagslane.dev/index.xml"

	LOGIN_EXPECT_ARGS    = 1
	REGISTER_EXPECT_ARGS = 1
	ADDFEED_EXPECT_ARGS  = 2
	FOLLOW_EXPECT_ARGS   = 1
	UNFOLLOW_EXPECT_ARGS = 1
	AGGR_EXPECT_ARGS     = 1
	BROWSE_DEFAULT_LIMIT = 2
)

type State struct {
	Db     *database.Queries
	Config *jsonconfig.Config
}

type Command struct {
	Name string
	Args []string
}

type Commands struct {
	CommandCallbacks map[string]func(*State, Command) error
}

var cliCommands Commands
var cliState State

func (cmds *Commands) register(name string, f func(*State, Command) error) {
	cmds.CommandCallbacks[name] = f
}

func (cmds *Commands) run(s *State, cmd Command) error {
	if callback, ok := cmds.CommandCallbacks[cmd.Name]; ok {
		return callback(s, cmd)
	}

	return fmt.Errorf("error: no callback function defined for %s", cmd.Name)
}

type RSSFeed struct {
	Channel struct {
		Title       string     `xml:"title"`
		Link        string     `xml:"link"`
		Description string     `xml:"description"`
		Item        []*RSSItem `xml:"item"`
	} `xml:"channel"`
}

type RSSItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
}

type Anchor struct {
	Href   string
	Target string
	Text   string
}

func main() {
	initGlobals()

	if len(os.Args) < MIN_ARGS {
		fmt.Printf("%v\n", fmt.Errorf("error: expecting at least one argument"))
		os.Exit(1)
	}

	args := os.Args[1:]
	currentCommandName := ""
	var currentCommandArgs []string = []string{}
	var userCommands []Command

	for _, arg := range args {
		if isDefinedCommandName(arg) {
			currentCommandName = arg
			if len(currentCommandArgs) > 0 {
				userCommands = append(userCommands, Command{Name: currentCommandName, Args: currentCommandArgs})
				currentCommandArgs = nil
			}
		} else if arg != "" {
			currentCommandArgs = append(currentCommandArgs, arg)
		}
	}

	if currentCommandName != "" && !commandAdded(currentCommandName, userCommands) {
		if currentCommandArgs == nil {
			currentCommandArgs = []string{}
		}
		userCommands = append(userCommands, Command{Name: currentCommandName, Args: currentCommandArgs})
	}

	for _, cmd := range userCommands {
		err := cliCommands.run(&cliState, cmd)
		if err != nil {
			fmt.Printf("%v\n", err)
			os.Exit(1)
		} else if cmd.Name == "reset" {
			os.Exit(0)
		}
	}
}

func commandAdded(name string, cmds []Command) bool {
	for _, c := range cmds {
		if c.Name == name {
			return true
		}
	}

	return false
}

func isDefinedCommandName(name string) bool {
	_, ok := cliCommands.CommandCallbacks[name]
	return ok
}

func initGlobals() {
	jsonConfig, err := jsonconfig.Read()

	if err != nil {
		fmt.Printf("%v\n", err)
		return
	}

	cliState.Config = jsonConfig

	db, err := sql.Open("postgres", cliState.Config.DbUrl)

	if err != nil {
		fmt.Printf("%v\n", fmt.Errorf("error: can't connect to gator database"))
	}

	cliState.Db = database.New(db)
	cliCommands = Commands{CommandCallbacks: make(map[string]func(*State, Command) error)}

	// Register each command for the CLI.
	cliCommands.register("login", handlerLogin)
	cliCommands.register("register", handlerRegister)
	cliCommands.register("reset", handlerReset)
	cliCommands.register("users", handlerUsers)
	cliCommands.register("agg", handlerAggregate)
	cliCommands.register("addfeed", isLoggedIn(handlerAddFeed))
	cliCommands.register("feeds", handlerFeeds)
	cliCommands.register("follow", isLoggedIn(handlerFollow))
	cliCommands.register("unfollow", isLoggedIn(handlerUnfollow))
	cliCommands.register("following", isLoggedIn(handlerFollowing))
	cliCommands.register("browse", isLoggedIn(handlerBrowse))
}

func handlerLogin(s *State, cmd Command) error {
	if len(cmd.Args) >= LOGIN_EXPECT_ARGS {
		_, err := s.Db.GetUserByName(context.Background(), cmd.Args[0])

		if err != nil {
			return fmt.Errorf("error: user %s does not exist", cmd.Args[0])
		}

		s.Config.SetUser(cmd.Args[0])
		fmt.Printf("User %s logged in.\n", s.Config.CurrentUsername)
		return nil
	} else {
		return fmt.Errorf("error: expected username but none specified")
	}
}

func handlerRegister(s *State, cmd Command) error {
	if len(cmd.Args) >= REGISTER_EXPECT_ARGS {
		_, err := s.Db.GetUserByName(context.Background(), cmd.Args[0])

		if err != nil {
			parms := database.CreateUserParams{ID: uuid.New(), CreatedAt: time.Now(), UpdatedAt: time.Now(), Name: cmd.Args[0]}

			s.Db.CreateUser(context.Background(), parms)
			fmt.Printf("Added user %s.\n", cmd.Args[0])
		} else {
			return fmt.Errorf("error: user %s already exists", cmd.Args[0])
		}

		err = handlerLogin(s, Command{Name: "login", Args: []string{cmd.Args[0]}})
		if err != nil {
			return err
		}

		return nil
	} else {
		return fmt.Errorf("error: expected name but none specified")
	}
}

func handlerReset(s *State, cmd Command) error {
	err := s.Db.DeleteAllUsers(context.Background())
	if err == nil {
		fmt.Println("Deleted all users.")
		return nil
	} else {
		return err
	}
}

func handlerUsers(s *State, cmd Command) error {
	users, err := s.Db.GetUsers(context.Background())
	if err != nil {
		return fmt.Errorf("error: couldn't get all users")
	}

	for _, user := range users {
		line := "* " + user.Name

		if s.Config.CurrentUsername == user.Name {
			line += " (current)"
		}

		fmt.Println(line)
	}

	return nil
}

func fetchFeed(ctx context.Context, feedURL string) (*RSSFeed, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", feedURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "gator")

	client := &http.Client{}
	resp, err := client.Do(req)

	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() // Ensure the response body is closed

	body, err := io.ReadAll(resp.Body)

	if err != nil {
		return nil, err
	}

	var feed RSSFeed

	// Ensure we're passing a pointer to destObject
	xmlErr := xml.Unmarshal(body, &feed)
	if xmlErr != nil {
		return nil, xmlErr
	}

	feed.Channel.Title = html.UnescapeString(feed.Channel.Title)
	feed.Channel.Description = html.UnescapeString(feed.Channel.Description)

	unescapeFeedStrings(&feed)

	return &feed, nil
}

func unescapeFeedStrings(feed *RSSFeed) {
	feed.Channel.Title = html.UnescapeString(feed.Channel.Title)
	feed.Channel.Description = html.UnescapeString(feed.Channel.Description)

	for _, item := range feed.Channel.Item {
		item.Title = html.UnescapeString(item.Title)
		item.Description = html.UnescapeString(item.Description)
	}
}

func printFeed(feed *RSSFeed) {
	fmt.Printf("      Title: %s\n", feed.Channel.Title)
	fmt.Printf("       Link: %s\n", feed.Channel.Link)
	fmt.Printf("Description: %s\n", feed.Channel.Description)

	for _, item := range feed.Channel.Item {
		fmt.Printf("\t      Title: %s\n", item.Title)
		fmt.Printf("\t       Link: %s\n", item.Link)
		fmt.Printf("\tDescription: %s\n", item.Description)
		fmt.Printf("\t   Publised: %s\n", item.PubDate)
		fmt.Println()
	}
}

func handlerAggregate(s *State, cmd Command) error {
	if len(cmd.Args) >= AGGR_EXPECT_ARGS {
		timeBetweenRequests, err := time.ParseDuration(cmd.Args[0])
		if err != nil {
			return err
		}

		fmt.Printf("Fetching feeds every %.1f minute(s).\n", timeBetweenRequests.Minutes())
		ticker := time.NewTicker(timeBetweenRequests)

		for ; ; <-ticker.C {
			fmt.Println("Scraping now...")
			err := scrapeFeeds(s)
			if err != nil {
				return err
			}
		}
	} else {
		return fmt.Errorf("error: expected time between requests but none specified")
	}
}

func handlerAddFeed(s *State, cmd Command, user database.User) error {
	if len(cmd.Args) >= ADDFEED_EXPECT_ARGS {
		parms := database.CreateFeedParams{ID: uuid.New(), CreatedAt: time.Now(), UpdatedAt: time.Now(), Name: cmd.Args[0], Url: cmd.Args[1], UserID: user.ID}
		createdFeed, err := s.Db.CreateFeed(context.Background(), parms)

		if err != nil {
			return err
		}

		fmt.Printf("Added feed '%s'.\n", cmd.Args[0])
		fmt.Printf("%v\n", createdFeed)

		handlerFollow(s, Command{Name: "follow", Args: []string{createdFeed.Url}}, user)

		return nil
	} else {
		return fmt.Errorf("error: expected feed name and url")
	}
}

func handlerFeeds(s *State, cmd Command) error {
	feeds, err := s.Db.GetFeeds(context.Background())
	if err != nil {
		return fmt.Errorf("error: couldn't get all feeds")
	}

	for _, feed := range feeds {
		creatingUser, err := s.Db.GetUser(context.Background(), feed.UserID)
		if err != nil {
			return fmt.Errorf("error: No user found for feed '%s'", feed.Name)
		}

		printDbFeed(feed, creatingUser.Name)
	}

	return nil
}

func handlerFollow(s *State, cmd Command, user database.User) error {
	if len(cmd.Args[0]) >= FOLLOW_EXPECT_ARGS {
		feed, err := s.Db.GetFeedByUrl(context.Background(), cmd.Args[0])
		if err != nil {
			return err
		}

		_, err = s.Db.CreateFeedFollow(context.Background(), database.CreateFeedFollowParams{ID: uuid.New(), CreatedAt: time.Now(), UpdatedAt: time.Now(), FeedID: feed.ID, UserID: user.ID})
		if err != nil {
			return err
		}

		fmt.Printf("'%s' now follows feed '%s'.\n", user.Name, feed.Name)
		return nil
	}

	return fmt.Errorf("error: expected url")
}

func handlerFollowing(s *State, cmd Command, user database.User) error {
	userFollows, err := s.Db.GetFeedFollowsForUser(context.Background(), user.ID)
	if err != nil {
		return err
	}

	fmt.Printf("List of feeds that %s follows:\n", user.Name)
	for _, follow := range userFollows {
		fmt.Printf("\t%s\n", follow)
	}

	return nil
}

func printDbFeed(feed database.Feed, creator string) {
	fmt.Printf("Title: %s\n", feed.Name)
	fmt.Printf("Url: %s\n", feed.Url)
	fmt.Printf("Created By: %s\n", creator)
	fmt.Println()
}

func isLoggedIn(handler func(s *State, cmd Command, user database.User) error) func(*State, Command) error {
	return func(s *State, cmd Command) error {
		user, err := s.Db.GetUserByName(context.Background(), s.Config.CurrentUsername)
		if err != nil {
			return err
		}

		return handler(s, cmd, user)
	}
}

func handlerUnfollow(s *State, cmd Command, user database.User) error {
	if len(cmd.Args) >= UNFOLLOW_EXPECT_ARGS {
		feed, err := s.Db.GetFeedByUrl(context.Background(), cmd.Args[0])
		if err != nil {
			return err
		}

		parms := database.DeleteFollowForUserParams{FeedID: feed.ID, UserID: user.ID}
		err = s.Db.DeleteFollowForUser(context.Background(), parms)
		if err != nil {
			return err
		}

		fmt.Printf("%s no longer follows the '%s' feed.", s.Config.CurrentUsername, feed.Name)
		return nil
	}

	return fmt.Errorf("error: expected feed url but none specified")
}

func scrapeFeeds(s *State) error {
	fetchedFeed, err := s.Db.GetNextFeedToFetch(context.Background())
	if err != nil {
		return err
	}

	err = cliState.Db.MarkFeedFetched(context.Background(), database.MarkFeedFetchedParams{ID: fetchedFeed.ID, UpdatedAt: time.Now()})
	if err != nil {
		return err
	}

	fetchedRSSFeed, err := fetchFeed(context.Background(), fetchedFeed.Url)
	if err != nil {
		return err
	}

	fmt.Printf("Items for feed '%s'\n", fetchedFeed.Name)
	if len(fetchedRSSFeed.Channel.Item) > 0 {
		for _, feedItem := range fetchedRSSFeed.Channel.Item {
			// If feed item with url already exists, move on to the next.
			_, urlNotAvailable := s.Db.GetPostByUrl(context.Background(), feedItem.Link)
			if urlNotAvailable == nil {
				continue
			} else {
				parsedDate, err := time.Parse("Mon, 02 Jan 2006 15:04:05 -0700", feedItem.PubDate)
				if err != nil {
					fmt.Printf("%s\n", fmt.Errorf("%v", err))
				}

				// If the description is an html anchor tag, split it appropriately.
				if strings.HasPrefix(feedItem.Description, "<a ") && strings.HasSuffix(feedItem.Description, "</a>") {
					anchor, err := extractAnchor(feedItem.Description)
					if err != nil {
						return err
					}

					feedItem.Description = anchor.Text
					feedItem.Link = anchor.Href
				}

				parms := database.CreatePostParams{ID: uuid.New(), CreatedAt: time.Now(), UpdatedAt: time.Now(), Title: feedItem.Title, Url: feedItem.Link, Description: feedItem.Description, PublishedAt: parsedDate, FeedID: fetchedFeed.ID}
				_, err = s.Db.CreatePost(context.Background(), parms)
				if err != nil {
					fmt.Printf("%s\n", fmt.Errorf("%v", err))
				}
			}
		}
	} else {
		fmt.Println("\tNo items for this feed.")
	}

	return nil
}

func extractAnchor(html string) (*Anchor, error) {
	// Load HTML string into goquery document
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, err
	}

	var anchor Anchor
	doc.Find("a").Each(func(i int, s *goquery.Selection) {
		link, _ := s.Attr("href")
		target, _ := s.Attr("target")

		anchor = Anchor{Href: link, Target: target, Text: s.Text()}
	})

	return &anchor, nil
}

func handlerBrowse(s *State, cmd Command, user database.User) error {
	resLimit := BROWSE_DEFAULT_LIMIT

	if len(cmd.Args) > 0 {
		lim, err := strconv.Atoi(cmd.Args[0])
		if err != nil {
			return err
		}

		resLimit = lim
	}

	parms := database.GetPostsForUserParams{UserID: user.ID, Limit: int32(resLimit)}
	userPosts, err := s.Db.GetPostsForUser(context.Background(), parms)
	if err != nil {
		return err
	}

	if len(userPosts) > 0 {
		fmt.Printf("List of posts %s follows:\n", user.Name)
		for _, post := range userPosts {
			printPost(post)
		}
	} else {
		fmt.Printf("%s does not follow any feeds.\n", s.Config.CurrentUsername)
	}

	return nil
}

func printPost(post database.Post) {
	fmt.Printf("\t    Post ID: %s\n", post.ID)
	fmt.Printf("\t      Title: %s\n", post.Title)
	fmt.Printf("\t        Url: %s\n", post.Url)
	fmt.Printf("\t  CreatedAt: %s\n", post.CreatedAt)
	fmt.Printf("\t  UpdatedAt: %s\n", post.UpdatedAt)
	fmt.Printf("\tDescription: %s\n", post.Description)
	fmt.Printf("\tPublishedAt: %s\n", post.PublishedAt)
	fmt.Printf("\t    Feed ID: %s\n", post.FeedID)
	fmt.Println()
}
