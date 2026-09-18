package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/CodeChefVIT/cookoff-11.0-be/internal/db/sqlc"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/dto"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/helpers/auth"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/helpers/utils"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/logging"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/labstack/echo/v4"
)

type AuthController struct {
	queries *sqlc.Queries
	client  *http.Client
}

func NewAuthController(queries *sqlc.Queries) *AuthController {
	return &AuthController{queries: queries, client: &http.Client{Timeout: 5 * time.Second}}
}

func (ac *AuthController) StartGoogle(c echo.Context) error {
	if utils.Config.GoogleClientID == "" || utils.Config.GoogleClientSecret == "" || utils.Config.GoogleRedirectURI == "" {
		return c.JSON(http.StatusInternalServerError, dto.NewErrorResponse("OAuth is not configured", nil))
	}
	state, cookie, err := auth.NewState()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, dto.NewErrorResponse("Unable to start OAuth", nil))
	}
	c.SetCookie(cookie)
	values := url.Values{
		"client_id":     {utils.Config.GoogleClientID},
		"redirect_uri":  {utils.Config.GoogleRedirectURI},
		"response_type": {"code"},
		"scope":         {"openid email profile"},
		"state":         {state},
	}
	return c.Redirect(http.StatusFound, utils.Config.GoogleAuthURL+"?"+values.Encode())
}

func (ac *AuthController) GoogleCallback(c echo.Context) error {
	stateCookie, err := c.Cookie(auth.StateCookie)
	validState := auth.ValidateState(stateCookie, c.QueryParam("state"))
	c.SetCookie(auth.ClearStateCookie())
	if err != nil || !validState || c.QueryParam("code") == "" {
		return loginFailed(c, "oauth_failed")
	}
	identity, err := ac.googleIdentity(c.Request().Context(), c.QueryParam("code"))
	if err != nil {
		logging.Errorf("Google authentication failed: %v", err)
		return loginFailed(c, "oauth_failed")
	}
	user, err := ac.findUser(c.Request().Context(), identity)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return loginFailed(c, "not_registered")
		}
		logging.Errorf("OAuth find user: %v", err)
		return loginFailed(c, "server_error")
	}
	if user.IsBanned {
		for _, cookie := range auth.ClearSessionCookies() {
			c.SetCookie(cookie)
		}

		return loginFailed(c, "banned")
	}
	cookies, err := auth.SessionCookies(auth.User{ID: user.ID.String(), Email: user.Email, Role: user.Role})
	if err != nil {
		logging.Errorf("OAuth session cookies: %v", err)
		return loginFailed(c, "server_error")
	}
	for _, cookie := range cookies {
		c.SetCookie(cookie)
	}
	return c.Redirect(http.StatusFound, redirectURL(user.Role))
}

func (ac *AuthController) RefreshToken(c echo.Context) error {
	cookie, err := c.Cookie(auth.RefreshCookie)
	if err != nil {
		return ac.unauthorized(c)
	}
	claims, err := auth.ParseToken(cookie.Value, auth.RefreshType)
	if err != nil {
		return ac.unauthorized(c)
	}
	id, err := uuid.Parse(claims.UserID)
	if err != nil {
		return ac.unauthorized(c)
	}
	user, err := ac.queries.GetUserByID(c.Request().Context(), id)
	if err != nil || user.IsBanned {
		return ac.unauthorized(c)
	}
	cookies, err := auth.SessionCookies(auth.User{ID: user.ID.String(), Email: user.Email, Role: user.Role})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, dto.NewCodedError("Unable to refresh session", dto.CodeInternal))
	}
	for _, sessionCookie := range cookies {
		c.SetCookie(sessionCookie)
	}
	return c.NoContent(http.StatusNoContent)
}

func (ac *AuthController) Logout(c echo.Context) error {
	for _, cookie := range auth.ClearSessionCookies() {
		c.SetCookie(cookie)
	}
	return c.NoContent(http.StatusNoContent)
}

func (ac *AuthController) unauthorized(c echo.Context) error {
	for _, cookie := range auth.ClearSessionCookies() {
		c.SetCookie(cookie)
	}

	return c.JSON(http.StatusUnauthorized, dto.NewCodedError("Unauthorized", dto.CodeUnauthorized))
}

type googleIdentity struct{ Subject, Email, Name string }

// googleBool accepts both JSON booleans and Google tokeninfo's string-encoded "true"/"false".
type googleBool bool

func (b *googleBool) UnmarshalJSON(data []byte) error {
	var value bool
	if err := json.Unmarshal(data, &value); err == nil {
		*b = googleBool(value)
		return nil
	}
	var text string
	if err := json.Unmarshal(data, &text); err != nil {
		return err
	}
	*b = googleBool(strings.EqualFold(text, "true"))
	return nil
}

func (ac *AuthController) googleIdentity(ctx context.Context, code string) (googleIdentity, error) {
	values := url.Values{"code": {code}, "client_id": {utils.Config.GoogleClientID}, "client_secret": {utils.Config.GoogleClientSecret}, "redirect_uri": {utils.Config.GoogleRedirectURI}, "grant_type": {"authorization_code"}}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, utils.Config.GoogleTokenURL, strings.NewReader(values.Encode()))
	if err != nil {
		return googleIdentity{}, err
	}
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
	response, err := ac.client.Do(request)
	if err != nil {
		return googleIdentity{}, err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return googleIdentity{}, fmt.Errorf("token exchange failed")
	}
	var token struct {
		IDToken string `json:"id_token"`
	}
	if decodeErr := json.NewDecoder(response.Body).Decode(&token); decodeErr != nil || token.IDToken == "" {
		return googleIdentity{}, fmt.Errorf("missing ID token")
	}
	infoURL := utils.Config.GoogleInfoURL + "?" + url.Values{"id_token": {token.IDToken}}.Encode()
	request, err = http.NewRequestWithContext(ctx, http.MethodGet, infoURL, nil)
	if err != nil {
		return googleIdentity{}, err
	}
	response, err = ac.client.Do(request)
	if err != nil {
		return googleIdentity{}, err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return googleIdentity{}, fmt.Errorf("invalid ID token")
	}
	var info struct {
		Subject       string     `json:"sub"`
		Email         string     `json:"email"`
		EmailVerified googleBool `json:"email_verified"`
		Name          string     `json:"name"`
		Audience      string     `json:"aud"`
		Issuer        string     `json:"iss"`
	}
	if err := json.NewDecoder(response.Body).Decode(&info); err != nil {
		return googleIdentity{}, err
	}
	if info.Subject == "" || info.Email == "" || !bool(info.EmailVerified) || info.Audience != utils.Config.GoogleClientID || (info.Issuer != "accounts.google.com" && info.Issuer != "https://accounts.google.com") {
		return googleIdentity{}, fmt.Errorf("unverified Google identity")
	}
	return googleIdentity{Subject: info.Subject, Email: info.Email, Name: info.Name}, nil
}

func (ac *AuthController) findUser(ctx context.Context, identity googleIdentity) (sqlc.User, error) {
	googleID := identity.Subject
	user, err := ac.queries.GetUserByGoogleID(ctx, &googleID)
	if err == nil {
		return user, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return sqlc.User{}, err
	}
	// No account claimed by this Google identity yet: check for a
	// pre-seeded account matching the verified email and link it.
	user, err = ac.queries.GetUserByEmail(ctx, identity.Email)
	if err != nil {
		return sqlc.User{}, err
	}
	linked, err := ac.queries.LinkGoogleID(ctx, sqlc.LinkGoogleIDParams{ID: user.ID, GoogleID: &googleID})
	if err != nil {
		return sqlc.User{}, err
	}
	return linked, nil
}

// loginFailed sends the browser back to the portal's login page with a
// reason code instead of stranding it on a JSON error from the API domain.
func loginFailed(c echo.Context, reason string) error {
	target := strings.TrimRight(utils.Config.FrontendURL, "/") + "/login?" + url.Values{"error": {reason}}.Encode()
	return c.Redirect(http.StatusFound, target)
}

func redirectURL(role string) string {
	if strings.ToLower(role) == "admin" {
		return strings.TrimRight(utils.Config.AdminURL, "/") + "/dashboard"
	}
	return strings.TrimRight(utils.Config.FrontendURL, "/")
}
