package utils

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/CodeChefVIT/cookoff-11.0-be/internal/logging"
	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

// please remove unnecessary
type cfg struct {
	JWTSecret          string        `env:"JWT_SECRET,notEmpty"`
	FrontendURL        string        `env:"FRONTEND_URL" envDefault:"http://localhost:3000"`
	AdminURL           string        `env:"ADMIN_URL" envDefault:"http://localhost:3067"`
	BackendURL         string        `env:"BACKEND_URL" envDefault:"http://localhost:8080"`
	CookieDomain       string        `env:"COOKIE_DOMAIN"`
	Env                string        `env:"ENV" envDefault:"development"`
	GoogleRedirectURI  string        `env:"GOOGLE_REDIRECT_URI"`
	GoogleClientID     string        `env:"GOOGLE_CLIENT_ID"`
	GoogleClientSecret string        `env:"GOOGLE_CLIENT_SECRET"`
	GoogleAuthURL      string        `env:"GOOGLE_AUTH_URL" envDefault:"https://accounts.google.com/o/oauth2/v2/auth"`
	GoogleTokenURL     string        `env:"GOOGLE_TOKEN_URL" envDefault:"https://oauth2.googleapis.com/token"`
	GoogleInfoURL      string        `env:"GOOGLE_INFO_URL" envDefault:"https://oauth2.googleapis.com/tokeninfo"`
	AccessTokenTTL     time.Duration `env:"ACCESS_TOKEN_TTL" envDefault:"15m"`
	RefreshTokenTTL    time.Duration `env:"REFRESH_TOKEN_TTL" envDefault:"48h"`

	Port             string `env:"PORT" envDefault:"8080"`
	PostgresHost     string `env:"POSTGRES_HOST,notEmpty"`
	PostgresPort     string `env:"POSTGRES_PORT,notEmpty"`
	PostgresUser     string `env:"POSTGRES_USER,notEmpty"`
	PostgresPassword string `env:"POSTGRES_PASSWORD,notEmpty"`
	PostgresDB       string `env:"POSTGRES_DB,notEmpty"`

	CookieSecure      bool   `env:"SECURE" envDefault:"false"`
	ElasticHost       string `env:"ELASTIC_HOST,notEmpty"`
	ElasticPort       int    `env:"ELASTIC_PORT" envDefault:"9200"`
	ElasticScheme     string `env:"ELASTIC_SCHEME" envDefault:"http"`
	ElasticUsername   string `env:"ELASTIC_USERNAME,notEmpty"`
	ElasticPassword   string `env:"ELASTIC_PASSWORD,notEmpty"`
	ElasticIndex      string `env:"ELASTIC_INDEX" envDefault:"logs"`
	ElasticMaxRetries int    `env:"ELASTIC_MAX_RETRIES" envDefault:"3"`
	ElasticInsecure   bool   `env:"ELASTIC_INSECURE" envDefault:"false"`
	RedisHost         string `env:"REDIS_HOST" envDefault:"127.0.0.1"`
	RedisPort         string `env:"REDIS_PORT" envDefault:"6379"`
	RedisPassword     string `env:"REDIS_PASSWORD"`

	CallbackURL string `env:"CALLBACK_URL"`
	// Shared secret appended to the Judge0 callback URL and required back on
	// PUT /judge0callback. That route cannot use the JWT middleware (Judge0 is
	// not a logged-in user), so without this anyone who guesses a submission
	// token can post a verdict. Optional: leave unset and the callback stays
	// open, exactly as before.
	Judge0CallbackSecret string `env:"JUDGE0_CALLBACK_SECRET"`
	Judge0URI            string `env:"JUDGE0_URI"`
}

var Config cfg

// CORSOrigins is a computed list of allowed frontend origins for CORS
var CORSOrigins []string

func LoadConfig() error {
	if err := godotenv.Load(); err != nil {
		logging.Infof("No .env file found, continuing with environment variables")
	}

	if err := env.Parse(&Config); err != nil {
		return fmt.Errorf("failed while trying to parse env: %+v", err)
	}
	if Config.JWTSecret == "" {
		return fmt.Errorf("JWT_SECRET must not be empty")
	}
	if Config.GoogleRedirectURI == "" || Config.GoogleClientID == "" || Config.GoogleClientSecret == "" {
		return fmt.Errorf("google OAuth configuration is incomplete")
	}
	if Config.AccessTokenTTL <= 0 || Config.RefreshTokenTTL <= 0 {
		return fmt.Errorf("token TTLs must be positive")
	}

	if Config.FrontendURL != "" {
		if u, err := url.Parse(strings.TrimRight(Config.FrontendURL, "/")); err == nil {
			if strings.EqualFold(u.Scheme, "https") {
				Config.CookieSecure = true
			}
		}
	}

	if Config.BackendURL != "" {
		if u, err := url.Parse(strings.TrimRight(Config.BackendURL, "/")); err == nil {
			if strings.EqualFold(u.Scheme, "https") {
				Config.CookieSecure = true
			}
			// auto derive cookie domain from backend url host if not explicitly set
			if Config.CookieDomain == "" {
				Config.CookieDomain = u.Hostname()
			}
		}
	}

	// Build CORS origins list from FRONTEND_URL and ADMIN_URL (if set).
	CORSOrigins = []string{}
	if strings.TrimSpace(Config.FrontendURL) != "" {
		CORSOrigins = append(CORSOrigins, strings.TrimRight(strings.TrimSpace(Config.FrontendURL), "/"))
	}
	if strings.TrimSpace(Config.AdminURL) != "" {
		CORSOrigins = append(CORSOrigins, strings.TrimRight(strings.TrimSpace(Config.AdminURL), "/"))
	}

	logging.Infof("Configuration successfully loaded")
	return nil
}
