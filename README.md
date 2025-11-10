# Goyco

[![Go Version](https://img.shields.io/badge/Go-1.25.0-blue.svg)](https://golang.org/)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-17-blue.svg)](https://www.postgresql.org/)
[![License](https://img.shields.io/badge/License-GPLv3-green.svg)](LICENSE)

Goyco is a Y Combinator-style news aggregation platform built in Go. It will allow you to host your own news aggregation platform, with a modern-ish UI and a fully functional REST API.

You have the flexibility to personalize the UI with your community’s name, and you can deploy Goyco on your own server, in the cloud or anywhere else you want. The rest of the features is described below.

It's free (as in free beer), open-source and sadly not (yet) fully customizable.

By the way, the web interface is living proof that I'm not a front-end developer — but hey, it loads! Please, don't judge me too harshly.

You can get a preview of the application through the [screenshots](screenshots).

## Architecture

### Technology Stack

It's basically pure Go (using Chi router), raw CSS and PostgreSQL 17.

## Quick Start

### Prerequisites

- Go 1.25.0 or later
- PostgreSQL 17 or later
- SMTP server for email functionality

### Setup PostgreSQL database and user

If you're not using a managed database service or a docker container, we wrote a script to help you setup a local PostgreSQL database along with the `goyco` user.

```bash
scripts/setup-postgres.sh
```

It'll prompt you for the password for the `goyco` user and then setup the database and user.

### Installation

In order to install Goyco on your system, you can use the following commands run as root:

```bash
make
make install
cp .env.example /opt/goyco/.env # edit it to add your own parameters
make migrations
```

This will:

- Create system user and group
- Install the binary to `/opt/goyco/bin`
- Install the static assets to `/opt/goyco/internal/static/`
- Install the templates to `/opt/goyco/internal/templates/`
- Install the license to `/usr/share/licenses/goyco/`
- Install the documentation to `/usr/share/doc/goyco/`
- Run database migrations

Finally, polish permissions and enable and start the service:

```bash
chown -R goyco:goyco /opt/goyco
systemctl enable --now goyco
```

### Deploy using Docker (compose)

```bash
# Build the image
make docker-image

# Run with Docker Compose (from project root)
docker compose --env-file .env -f docker/compose.prod.yml up -d

# migrate the database
docker compose --env-file .env -f docker/compose.prod.yml exec app goyco migrate
```

Once you built the image, you can also run the docker container itself with right environment variables:

```bash
docker run -d --name goyco -p 8080:8080 --env-file .env --restart unless-stopped goyco:latest
```

## Configuration

Goyco uses environment variables for configuration.

Key settings include:

### Database Configuration

```bash
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=your_password
DB_NAME=goyco
DB_SSLMODE=disable
```

### Server Configuration

```bash
SERVER_HOST=0.0.0.0
SERVER_PORT=8080
```

### JWT Configuration

```bash
JWT_SECRET=your-secure-secret-key
JWT_EXPIRATION=1
JWT_REFRESH_EXPIRATION=168
```

### SMTP Configuration

```bash
SMTP_HOST=smtp.example.com
SMTP_PORT=587
SMTP_USERNAME=your-email@example.com
SMTP_PASSWORD=your-password
SMTP_FROM=noreply@example.com
```

Be sure to check `.env.example` for more details.

### Reverse Proxy Configuration

To use a reverse proxy in order to offload the SSL termination (for example), here's a sample nginx configuration:

```nginx
upstream goyco {
   server 10.200.1.11:8080;
}

server {
    listen 443 ssl;
    server_name goyco.example.com;

    ssl_certificate /etc/letsencrypt/live/goyco.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/goyco.example.com/privkey.pem;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_prefer_server_ciphers on;
    ssl_ciphers 'ECDHE+AESGCM:CHACHA20';

    location / {
        proxy_pass http://goyco;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-Proto https;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

### Application Settings

```bash
APP_BASE_URL=https://goyco.example.com # assuming you are using a reverse proxy
ADMIN_EMAIL=admin@example.com
TITLE=Goyco # will be displayed in the web interface, choose wisely
DEBUG=false
```

## API Documentation

The API is fully documented with Swagger.

Once running, visit:

- **Swagger UI**: `https://goyco.example.com/swagger/index.html`

You can also use `curl` to get the API info, health check and even metrics:

```bash
curl -X GET https://goyco.example.com/api
curl -X GET https://goyco.example.com/health
curl -X GET https://goyco.example.com/metrics
```

You can also use `jq` to parse the JSON responses:

```bash
curl -X GET https://goyco.example.com/api | jq
curl -X GET https://goyco.example.com/health | jq
curl -X GET https://goyco.example.com/metrics | jq
```

It'll be more readable and easier to parse.

### Key Endpoints

#### Authentication

- `POST /api/auth/register` - Register new user
- `POST /api/auth/login` - Login user
- `GET /api/auth/confirm` - Confirm email
- `POST /api/auth/logout` - Logout user

#### Posts

- `GET /api/posts` - List posts
- `POST /api/posts` - Create post
- `GET /api/posts/{id}` - Get specific post
- `PUT /api/posts/{id}` - Update post
- `DELETE /api/posts/{id}` - Delete post

#### Voting

- `POST /api/posts/{id}/vote` - Cast vote
- `DELETE /api/posts/{id}/vote` - Remove vote
- `GET /api/posts/{id}/votes` - Get post votes

## CLI Commands

Goyco includes a comprehensive CLI for administration:

```bash
# Server management
./bin/goyco run                    # Run server in foreground
./bin/goyco start                  # Start server as daemon
./bin/goyco stop                   # Stop daemon
./bin/goyco status                 # Check server status

# Database management
./bin/goyco migrate                # Run database migrations
./bin/goyco seed database          # Seed database with sample data

# User management
./bin/goyco user create            # Create new user
./bin/goyco user list              # List users
./bin/goyco user update            # Update user
./bin/goyco user delete            # Delete user
./bin/goyco user lock              # Lock user
./bin/goyco user unlock            # Unlock user

# Post management
./bin/goyco post list              # List posts
./bin/goyco post search            # Search posts
./bin/goyco post delete            # Delete post

# Maintenance
./bin/goyco prune posts             # Hard delete posts of deleted users
./bin/goyco prune users             # Hard delete users
./bin/goyco prune all               # Hard delete all users and posts
```

## Development

### Get the sources

```bash
git clone https://github.com/sandrocazzaniga/goyco.git
cd goyco
```

Note: if you mean to contribute to the project, please fork the repository first.

### Create a `.env` file

```bash
cp .env.example .env
```

Customize the `.env` file to add your own parameters.

Here's the SMTP configuration for `mailpit` (for development purposes):

```bash
# SMTP Configuration
SMTP_HOST=localhost
SMTP_PORT=1025
SMTP_FROM=noreply@goyco.xiz
```

While you're hacking around, be sure to set `SERVER_HOST` and `SERVER_PORT` in order to be able to access the application from your browser. Also, beware of `APP_BASE_URL` parameter.

### Install and manage development dependencies

```bash
make build-deps
```

It will start a PostgreSQL database and a [mailpit](https://mailpit.axllent.org/) server in order to test the application.

The web front of mailpit server will be available at `http://localhost:8025` and will allow you to view the emails sent by the application. No matter the recipient, all emails will be captured by `mailpit`.

Once you're done, you can use `make clean-deps` to stop the dependencies and remove the containers and volumes.

### Build the application

```bash
make
```

The build process will create the binary in the `bin/` directory.

Then, make the migrations:

```bash
./bin/goyco migrate
```

It will create the necessary tables in the database.

### Run the application

```bash
./bin/goyco run
```

It will start the application in development mode. You can also run it as a daemon:

```bash
./bin/goyco start
```

Then, use `./bin/goyco` to manage the application and notably to seed the database with sample data.

### Project Structure

```bash
goyco/
├── bin/ # Compiled binaries (created after build)
├── cmd/
│ └── goyco/ # Main CLI application entrypoint
├── docker/ # Docker Compose & related files
├── docs/ # Documentation and API specs
├── internal/
│ ├── config/ # Configuration management
│ ├── database/ # Database models and access
│ ├── dto/ # Data Transfer Objects (DTOs)
│ ├── e2e/ # End-to-end tests
│ ├── fuzz/ # Fuzz tests
│ ├── handlers/ # HTTP handlers
│ ├── integration/ # Integration tests
│ ├── middleware/ # HTTP middleware
│ ├── repositories/ # Data access layer
│ ├── security/ # Security and auth logic
│ ├── server/ # HTTP server implementation
│ ├── services/ # Business logic
│ ├── static/ # Static web assets
│ ├── templates/ # HTML templates
│ ├── testutils/ # Test helpers/utilities
│ ├── validation/ # Input validation
│ └── version/ # Version information
├── scripts/ # Utility/maintenance scripts
├── services/
│ └── goyco.service # Systemd service unit example
├── .env.example # Environment variable example
├── AUTHORS # Authors file
├── Dockerfile # Docker build file
├── LICENSE # License file
├── Makefile # Project build/test targets
└── README.md # This file
```

### Testing

```bash
# Run all tests
make test

# Run specific test suites
make unit-tests
make integration-tests
make e2e-tests

# Run fuzz testing (can take a bit of CPU and time)
make fuzz-tests
```

### Code Quality

```bash
# Format code
make format

# Run linter
make lint
```

### Regerenate Swagger documentation

If you make changes to the API, you can regenerate the swagger documentation by running the following command after modifying the swagger annotations:

```bash
# Regenerate Swagger documentation
make swagger
```

This will regenerate the swagger documentation and update the `docs/swagger.json` and `docs/swagger.yaml` files.

## Roadmap

- [ ] migrate cli to urfave/cli
- [ ] add a ML powered nsfw link detection
- [ ] add right management within the app
- [ ] add an admin backoffice to manage rights, users, content and settings
- [ ] add a way to run read-only communities
- [ ] use tailwind instead of raw css
- [ ] kubernetes deployment
- [ ] store configuration in the database

## Contributing

Feedbacks are welcome!

But as it's a personal gitea and you cannot create accounts, feel free to contact me at <sandro@cazzaniga.fr> to get one.

Once you have it, follow the usual workflow:

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests for new functionality
5. Ensure all tests pass
6. Submit a pull request

Then, I'll review your changes and merge them if they are good.

## License

This project is licensed under the GNU General Public License v3.0 or later (GPLv3+). See the [LICENSE](LICENSE) file for details.

---

**Goyco** - A modern news aggregation platform built with Go, PostgreSQL and most importantly, love.
