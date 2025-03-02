
# Gator Aggrgation CLI Tool
This project is a aggregation tool that is meant to showcase basics of Database desgin, CLI fundamentals, and database migrations.

This tool uses a Go for the CLI logic, a Postgres database, goose database versioning, and sqlc to generate boilerplate database interaction logic for the Go CLI to use to interact with the database.

# Installation
A few things are required to run this tool. How to install them are explained below.

## Installing Goose
To install goose for database migrations, you can run go install github.com/pressly/goose/v3/cmd/goose@latest in your terminal. 
Run goose -version afterwards to ensure it is installed.

## Installing PostgresSQL
### On MacOS with brew:
Run the following command in your terminal:
* brew install postgresql@15
#### Linux / WSL (Debian)
Run the following commands in your terminal:
* sudo apt update
* sudo apt install postgresql postgresql-contrib
* sudo passwd postgres (to set your password for the db)

Then run psql --version to ensure that the installation was successful.

### Starting the Postgres Server (Optional)
#### On MacOS with brew:
brew service start postgresql@15
#### Linux
sudo service postgresql start

### Entering the psql Shell (Optional) 
#### MacOs
psql postgres
#### Linux
sudo -u postgres psql
##### Exiting psql
To exit the psql shell at any time, simply type exit

## Installing the Gator CLI Tool
Simply type go install gator to install the CLI.
to run the Gator CLI, type ./gator followed by the commands and the arguments the command expected. Commands are explained below.

## Gator CLI Commands
Below is the usage of the various commands that can be used in the CLI tool. Commands that expect additional arguments will be surrounded with the <> characters. e.g. <some_required_parameter> If an argument is optional, it will be surrounded with []. e.g. [some_optional_parameter]

### Example Command Execution
./gator <command> <parameters>
Where the command and its arguments are defined below.

### Register 
The register command adds a user with the given name to the database. There cannot be more than one user with the same name. Once the user is registered, they are logged in as the active user.
#### Parameters
*name* The name of the user to register
#### Syntax
register <name to register>
#### Example Usage
register testuser1

### Login 
The login command logs in a user that has already been registered.
#### Parameters
*name* The name of the user to login.
#### Syntax
login <name to login>
#### Example Usage
login testuser1

### Reset 
The reset command removes all users along with all their feeds and posts they created.
#### Parameters
None
#### Syntax
reset
#### Example Usage
reset

### Users 
The users command lists all the users currently registered in the database. The user that is currently logged in will have the '(current)' text beside their name in the list.
#### Parameters
None
#### Syntax
users 
#### Example Usage
users

### Aggregate 
The aggregate command fetches the most out of date feed and updates it. This function will continually fetch feeds in time increments defined by the required time between requests parameter. 
**IMPORTANT** This command is terminated by pressing Ctrl (or Command if on MacOS) + C.
This command is best run in a separate terminal while another terminal instance runs the gator program and uses the other commands.
#### Parameters
*time_between_aggregations* a time string that defines the time to wait between feed updates. e.g. 1m, 30s, 1h, etc.
#### Syntax
agg <time_between_aggregations>
#### Example usage
agg 1m (for updates every minute.)

### Add Feed 
The addfeed command is used to add a new feed to the database. The current user is added as its creator, and the creator is automatically set as a follower of the feed. 
#### Parameters
*feed name* The name of the feed being added.
*url* The destination url that contains the feed content.
#### Syntax
addfeed <feed_name> <url>
#### Example usage
addfeed "Example Feed" "https://www.example.com/rss"

### Feeds 
The feeds command lists all the feeds currently registered in the database.
#### Parameters
None
#### Syntax
feeds
#### Example Usage
feeds

### Follow
The follow command looks up the a feed supplied by the url parameter and adds the current user as a follower.
#### Parameters
*url* The url for the feed to follow.
#### Syntax
follow <url>
#### Example Usage
follow https://www.example.com/rss

### Following
The following command looks up the list of feeds the current user follows and prints them to the terminal.
#### Parameters
None
#### Syntax
following
#### Example Usage
following

### UNfollow
The unfollow is the opposite of the follow command. It looks up the a feed supplied by the url parameter and followed by the current user and unfollows the user from the feed.
#### Parameters
*url* The url for the feed to unfollow.
#### Syntax
unfollow <url>
#### Example Usage
unfollow https://www.example.com/rss
