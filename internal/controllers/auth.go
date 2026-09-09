package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/CodeChefVIT/cookoff-11.0-be/internal/auth"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/db/sqlc"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/utils"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/labstack/echo/v4"
)

type AuthController struct {
	queries *sqlc.Queries
	client  *http.Client
}

func NewAuthController(queries *sqlc.Queries) *AuthController {
	return &AuthController{queries: queries, client: http.DefaultClient}
}

func (ac *AuthController) StartGoogle(c echo.Context) error {
	portal := c.QueryParam("portal")
	if portal != "admin" && portal != "participant" {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid portal")
	}
	if utils.Config.GoogleClientID == "" || utils.Config.GoogleClientSecret == "" || utils.Config.GoogleRedirectURI == "" {
		return echo.NewHTTPError(http.StatusInternalServerError, "OAuth is not configured")
	}
	state, cookie, err := auth.NewState(portal)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "unable to start OAuth")
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
	portal, validState := auth.ValidateState(stateCookie, c.QueryParam("state"))
	c.SetCookie(auth.ClearStateCookie())
	if err != nil || !validState || c.QueryParam("code") == "" {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid OAuth callback")
	}
	identity, err := ac.googleIdentity(c.Request().Context(), c.QueryParam("code"))
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "Google authentication failed")
	}
	user, err := ac.findOrCreateUser(c.Request().Context(), identity)
	if err != nil {
		if errors.Is(err, ErrEmailAlreadyUsed) {
			return echo.NewHTTPError(http.StatusConflict, "account cannot be linked")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "unable to establish session")
	}
	if user.IsBanned {
		for _, cookie := range auth.ClearSessionCookies() {
			c.SetCookie(cookie)
		}
		return echo.NewHTTPError(http.StatusForbidden, "account is banned")
	}
	cookies, err := auth.SessionCookies(auth.User{ID: user.ID.String(), Email: user.Email, Role: user.Role})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "unable to establish session")
	}
	for _, cookie := range cookies {
		c.SetCookie(cookie)
	}
	return c.Redirect(http.StatusFound, portalURL(portal))
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
		return echo.NewHTTPError(http.StatusInternalServerError, "unable to refresh session")
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
	return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
}

type googleIdentity struct{ Subject, Email, Name string }

var ErrEmailAlreadyUsed = errors.New("email already belongs to another account")

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
		Subject       string `json:"sub"`
		Email         string `json:"email"`
		EmailVerified string `json:"email_verified"`
		Name          string `json:"name"`
		Audience      string `json:"aud"`
		Issuer        string `json:"iss"`
	}
	if err := json.NewDecoder(response.Body).Decode(&info); err != nil {
		return googleIdentity{}, err
	}
	if info.Subject == "" || info.Email == "" || info.EmailVerified != "true" || info.Audience != utils.Config.GoogleClientID || (info.Issuer != "accounts.google.com" && info.Issuer != "https://accounts.google.com") {
		return googleIdentity{}, fmt.Errorf("unverified Google identity")
	}
	return googleIdentity{Subject: info.Subject, Email: info.Email, Name: info.Name}, nil
}

func (ac *AuthController) findOrCreateUser(ctx context.Context, identity googleIdentity) (sqlc.User, error) {
	googleID := identity.Subject
	user, err := ac.queries.GetUserByGoogleID(ctx, &googleID)
	if err == nil {
		return user, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return sqlc.User{}, err
	}
	id := uuid.New()
	user, err = ac.queries.CreateGoogleUser(ctx, sqlc.CreateGoogleUserParams{ID: id, Email: identity.Email, RegNo: "oauth_" + id.String(), Role: "user", GoogleID: &googleID, Name: identity.Name})
	var dbErr *pgconn.PgError
	if errors.As(err, &dbErr) && dbErr.Code == "23505" {
		return sqlc.User{}, ErrEmailAlreadyUsed
	}
	return user, err
}

func portalURL(portal string) string {
	if portal == "admin" {
		return strings.TrimRight(utils.Config.AdminURL, "/")
	}
	return strings.TrimRight(utils.Config.FrontendURL, "/")
}
