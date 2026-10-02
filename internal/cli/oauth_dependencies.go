package cli

import (
	"context"
	"time"
)

// OAuth adapters belong to one invocation; tests never replace shared functions.
type oauthDependencies struct {
	login            func(*Context, oauthConfig) (oauthToken, error)
	deviceLogin      func(*Context, oauthConfig) (oauthToken, error)
	random           func(int) (string, error)
	authorizationURL func(oauthConfig, string, string) (string, error)
	openBrowser      func(string) error
	waitForCode      func(context.Context, oauthConfig, string, time.Duration) (string, error)
	exchangeToken    func(context.Context, oauthConfig, string, string) (oauthToken, error)
}

func (ctx *Context) oauthDeps() oauthDependencies {
	d := ctx.oauth
	if d.login == nil {
		d.login = authOAuthLogin
	}
	if d.deviceLogin == nil {
		d.deviceLogin = authOAuthDeviceLogin
	}
	if d.random == nil {
		d.random = generateOAuthRandom
	}
	if d.authorizationURL == nil {
		d.authorizationURL = buildOAuthAuthorizationURL
	}
	if d.openBrowser == nil {
		d.openBrowser = openOAuthBrowser
	}
	if d.waitForCode == nil {
		d.waitForCode = waitForOAuthCode
	}
	if d.exchangeToken == nil {
		d.exchangeToken = exchangeOAuthToken
	}
	return d
}
