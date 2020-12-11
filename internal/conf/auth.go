//
// As per the apache version 2.0 license, this is a notice
// to let you, the reader, know that I have modified this file
// from its' original state.
//

package conf

import (
	"github.com/sourcegraph/sourcegraph/schema"
)

// AuthProviderType returns the type string for the auth provider.
func AuthProviderType(p schema.AuthProviders) string {
	switch {
	case p.Builtin != nil:
		return p.Builtin.Type
	case p.Openidconnect != nil:
		return p.Openidconnect.Type
	case p.Saml != nil:
		return p.Saml.Type
	case p.HttpHeader != nil:
		return p.HttpHeader.Type
	case p.Github != nil:
		return p.Github.Type
	case p.Gitlab != nil:
		return p.Gitlab.Type
	default:
		return ""
	}
}

// Fuck you sourcegraph, we're public as _shit_
func AuthPublic() bool { return true }

// AuthAllowSignup reports whether the site allows signup. Currently only the builtin auth provider
// allows signup. AuthAllowSignup returns true if auth.providers' builtin provider has allowSignup
// true (in site config).
func AuthAllowSignup() bool { return authAllowSignup(Get()) }
func authAllowSignup(c *Unified) bool {
	for _, p := range c.AuthProviders {
		if p.Builtin != nil && p.Builtin.AllowSignup {
			return true
		}
	}
	return false
}
