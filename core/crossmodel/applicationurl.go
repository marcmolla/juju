// Copyright 2015 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package crossmodel

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/juju/errors"
	"gopkg.in/juju/names.v2"
)

// ApplicationURL represents the location of an offered application and its
// associated exported endpoints.
type ApplicationURL struct {
	// Source represents where the offer is hosted.
	// If empty, the model is another model in the same controller.
	Source string // "<controller-name>" or "<jaas>" or ""

	// User is the user whose namespace in which the offer is made.
	// Where a model is specified, the user is the model owner.
	User string

	// ModelName is the name of the model providing the exported endpoints.
	// It is only used for local URLs or for specifying models in the same
	// controller.
	ModelName string

	// ApplicationName is the name of the application providing the exported endpoints.
	ApplicationName string
}

// Path returns the path component of the URL.
func (u *ApplicationURL) Path() string {
	var parts []string
	if u.User != "" {
		parts = append(parts, u.User)
	}
	if u.ModelName != "" {
		parts = append(parts, u.ModelName)
	}
	path := strings.Join(parts, "/")
	path = fmt.Sprintf("%s.%s", path, u.ApplicationName)
	if u.Source == "" {
		return path
	}
	return fmt.Sprintf("%s:%s", u.Source, path)
}

func (u *ApplicationURL) String() string {
	return u.Path()
}

// HasEndpoint returns whether this application URL includes an
// endpoint name in the application name.
func (u *ApplicationURL) HasEndpoint() bool {
	return strings.Contains(u.ApplicationName, ":")
}

// modelApplicationRegexp parses urls of the form controller:user/model.application[:relname]
var modelApplicationRegexp = regexp.MustCompile(`(/?((?P<user>[^/]+)/)?(?P<model>[^.]*)(\.(?P<application>[^:]*(:.*)?))?)?`)

//var modelApplicationRegexp = regexp.MustCompile(`(/?((?P<user>[a-zA-Z]+)/)?(?P<model>[a-zA-Z]+)?(\.(?P<application>[^:]*(:[a-zA-Z]+)?))?)?`)

// ParseApplicationURL parses the specified URL string into an ApplicationURL.
// The URL string is of one of the forms:
//  <model-name>.<application-name>
//  <model-name>.<application-name>:<relation-name>
//  <user>/<model-name>.<application-name>
//  <user>/<model-name>.<application-name>:<relation-name>
//  <controller>:<user>/<model-name>.<application-name>
//  <controller>:<user>/<model-name>.<application-name>:<relation-name>
func ParseApplicationURL(urlStr string) (*ApplicationURL, error) {
	return parseApplicationURL(urlStr)
}

// parseApplicationURL parses the specified URL string into an ApplicationURL.
func parseApplicationURL(urlStr string) (*ApplicationURL, error) {
	urlParts, err := parseApplicationURLParts(urlStr, false)
	if err != nil {
		return nil, err
	}
	url := ApplicationURL(*urlParts)
	return &url, nil
}

// ApplicationURLParts contains various attributes of a URL.
type ApplicationURLParts ApplicationURL

// ParseApplicationURLParts parses a partial URL, filling out what parts are supplied.
// This method is used to generate a filter used to query matching application URLs.
func ParseApplicationURLParts(urlStr string) (*ApplicationURLParts, error) {
	return parseApplicationURLParts(urlStr, true)
}

var endpointRegexp = regexp.MustCompile(`^[a-zA-Z0-9]+$`)

func maybeParseSource(urlStr string) (source, rest string) {
	parts := strings.Split(urlStr, ":")
	switch len(parts) {
	case 3:
		return parts[0], parts[1] + ":" + parts[2]
	case 2:
		if endpointRegexp.MatchString(parts[1]) {
			return "", urlStr
		}
		return parts[0], parts[1]
	}
	return "", urlStr
}

func parseApplicationURLParts(urlStr string, allowIncomplete bool) (*ApplicationURLParts, error) {
	var result ApplicationURLParts
	source, urlParts := maybeParseSource(urlStr)

	valid := !strings.HasPrefix(urlStr, ":")
	valid = valid && modelApplicationRegexp.MatchString(urlParts)
	if valid {
		result.Source = source
		result.User = modelApplicationRegexp.ReplaceAllString(urlParts, "$user")
		result.ModelName = modelApplicationRegexp.ReplaceAllString(urlParts, "$model")
		result.ApplicationName = modelApplicationRegexp.ReplaceAllString(urlParts, "$application")
	}
	if !valid || strings.Contains(result.ModelName, "/") || strings.Contains(result.ApplicationName, "/") {
		// TODO(wallyworld) - update error message when we support multi-controller and JAAS CMR
		return nil, errors.Errorf("application offer URL has invalid form, must be [<user/]<model>.<appname>: %q", urlStr)
	}
	if !allowIncomplete && result.ModelName == "" {
		return nil, errors.Errorf("application offer URL is missing model")
	}
	if !allowIncomplete && result.ApplicationName == "" {
		return nil, errors.Errorf("application offer URL is missing application")
	}

	// Application name part may contain a relation name part, so strip that bit out
	// before validating the name.
	appName := strings.Split(result.ApplicationName, ":")[0]
	// Validate the resulting URL part values.
	if result.User != "" && !names.IsValidUser(result.User) {
		return nil, errors.NotValidf("user name %q", result.User)
	}
	if result.ModelName != "" && !names.IsValidModelName(result.ModelName) {
		return nil, errors.NotValidf("model name %q", result.ModelName)
	}
	if appName != "" && !names.IsValidApplication(appName) {
		return nil, errors.NotValidf("application name %q", appName)
	}
	return &result, nil
}
