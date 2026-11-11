// Package updates checks published helper releases without downloading or installing them.
package updates

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/mod/semver"
)

// Version is set by scripts/build.sh from VERSION. Direct go builds are identified
// as development builds rather than claiming a release version.
var Version = "development"

const releasesAPI = "https://api.github.com/repos/jimbruges/arduino-flasher-ui-helper/releases?per_page=100"
const ReleasesURL = "https://github.com/jimbruges/arduino-flasher-ui-helper/releases"

type Info struct {
	Current    string `json:"current"`
	Latest     string `json:"latest"`
	Available  bool   `json:"available"`
	Comparable bool   `json:"comparable"`
	Preview    bool   `json:"preview"`
}

type release struct {
	Tag        string `json:"tag_name"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
}

func ReleaseURL(tag string) (string, error) {
	if !semver.IsValid(tag) {
		return "", fmt.Errorf("invalid release version")
	}
	return ReleasesURL + "/tag/" + url.PathEscape(tag), nil
}

func Check(ctx context.Context) (Info, error) {
	return check(ctx, &http.Client{Timeout: 8 * time.Second}, releasesAPI, BuildVersion())
}

func check(ctx context.Context, client *http.Client, endpoint, current string) (Info, error) {
	info := Info{Current: current, Comparable: semver.IsValid(current)}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return info, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "arduino-flasher-ui-helper/"+current)
	response, err := client.Do(req)
	if err != nil {
		return info, fmt.Errorf("could not reach GitHub: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		if response.StatusCode == http.StatusForbidden || response.StatusCode == http.StatusTooManyRequests {
			return info, fmt.Errorf("GitHub temporarily limited update checks; try again later")
		}
		return info, fmt.Errorf("GitHub update check returned HTTP %d", response.StatusCode)
	}
	var releases []release
	if err := json.NewDecoder(io.LimitReader(response.Body, 2<<20)).Decode(&releases); err != nil {
		return info, fmt.Errorf("could not read GitHub releases: %w", err)
	}
	includePreviews := !info.Comparable || semver.Prerelease(current) != ""
	for _, r := range releases {
		if r.Draft || !semver.IsValid(r.Tag) {
			continue
		}
		preview := r.Prerelease || semver.Prerelease(r.Tag) != ""
		if preview && !includePreviews {
			continue
		}
		if info.Latest == "" || semver.Compare(r.Tag, info.Latest) > 0 {
			info.Latest = r.Tag
			info.Preview = preview
		}
	}
	if info.Latest == "" {
		return info, fmt.Errorf("no published releases found for this update channel")
	}
	info.Available = info.Comparable && semver.Compare(info.Latest, current) > 0
	return info, nil
}

func BuildVersion() string { return strings.TrimSpace(Version) }
