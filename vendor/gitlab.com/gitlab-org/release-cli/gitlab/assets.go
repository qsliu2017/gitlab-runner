package gitlab

import (
	"encoding/json"
	"fmt"

	"gitlab.com/gitlab-org/release-cli/internal/flags"
)

// ParseAssets generates an instance of Asset from names and urls
func ParseAssets(names, urls, assetsLink []string) (*Assets, error) {
	// --assets-link takes precedence over --assets-links-name and --assets-link-url
	if len(assetsLink) > 0 {
		return parseAssetsLinkJSON(assetsLink)
	}

	if len(names) != len(urls) {
		return nil, fmt.Errorf("mismatch length --%s (%d) and --%s (%d) should be equal",
			flags.AssetsLinksName,
			len(names),
			flags.AssetsLinksURL,
			len(urls),
		)
	}

	if names == nil {
		return nil, nil
	}

	assets := &Assets{
		Links: make([]*Link, len(names)),
	}

	for k, name := range names {
		assets.Links[k] = &Link{
			Name: name,
			URL:  urls[k],
		}
	}

	return assets, nil
}

func parseAssetsLinkJSON(assetsLink []string) (*Assets, error) {
	assets := &Assets{
		Links: make([]*Link, len(assetsLink)),
	}

	for k, entry := range assetsLink {
		var link Link
		if err := json.Unmarshal([]byte(entry), &link); err != nil {
			return nil, fmt.Errorf("invalid JSON: %q", entry)
		}

		assets.Links[k] = &link
	}

	return assets, nil
}
