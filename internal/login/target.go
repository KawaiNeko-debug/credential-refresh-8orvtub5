package login

import (
	"errors"
	"net/url"
	"strings"
)

type Target struct {
	LoginURL                 string
	PasswordLoginURL         string
	CheckLoginURL            string
	SessionURL               string
	WebLoginByCodeURL        string
	MobileLoginByCodeURL     string
	UserInfoURL              string
	CookieURLs               []string
	WebOrigin                string
	MobileOrigin             string
	TicketCookieName         string
	PrimarySessionCookieName string
	GroupSessionCookieName   string
	CustomerCookieName       string
	CustomerCookieAlias      string
	WebAppID                 string
	MobileAppID              string
	MobileAccessHeader       string
	ClientTypeHeader         string
}

func TargetFromMarker(marker string) (Target, error) {
	marker = strings.ToLower(strings.TrimSpace(marker))
	if marker == "" {
		return Target{}, errors.New("TARGET_MARKER is required")
	}
	if strings.ContainsAny(marker, ".:/\\@") {
		return Target{}, errors.New("TARGET_MARKER must contain only the hidden domain label")
	}
	upper := strings.ToUpper(marker)
	domain := marker + ".com"
	passportOrigin := "https://passport." + domain
	webOrigin := "https://www." + domain
	mobileOrigin := "https://m." + domain
	memberOrigin := "https://member." + domain
	primarySessionCookie := "PROD-" + upper + "-CAS-SID"
	groupSessionCookie := upper + "GROUP_SESSIONID"

	for _, rawURL := range []string{passportOrigin, webOrigin, mobileOrigin, memberOrigin} {
		if _, err := url.ParseRequestURI(rawURL); err != nil {
			return Target{}, err
		}
	}
	return Target{
		LoginURL:                 passportOrigin + "/login",
		PasswordLoginURL:         passportOrigin + "/api/cas/login/with-password",
		CheckLoginURL:            passportOrigin + "/api/cas/sso/check-login",
		SessionURL:               webOrigin + "/api/portal/v1/secret/update",
		WebLoginByCodeURL:        webOrigin + "/login/login-by-code",
		MobileLoginByCodeURL:     mobileOrigin + "/api/login/login-by-code",
		UserInfoURL:              webOrigin + "/api/integral/user/getUserInfo",
		CookieURLs:               []string{passportOrigin + "/", webOrigin + "/", memberOrigin + "/"},
		WebOrigin:                webOrigin,
		MobileOrigin:             mobileOrigin,
		TicketCookieName:         "tgc",
		PrimarySessionCookieName: primarySessionCookie,
		GroupSessionCookieName:   groupSessionCookie,
		CustomerCookieName:       "customerCode",
		CustomerCookieAlias:      upper + "_CUSTOMER_CODE",
		WebAppID:                 upper + "_PORTAL_PC",
		MobileAppID:              upper + "_MOBILE_APP",
		MobileAccessHeader:       "X-" + upper + "-AccessToken",
		ClientTypeHeader:         "X-" + upper + "-ClientType",
	}, nil
}
