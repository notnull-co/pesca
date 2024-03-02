package registry

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/notnull-co/dynaclient/pkg/client"
	"github.com/notnull-co/pesca/internal/domain"
	"github.com/rs/zerolog/log"
)

const (
	apiVersion         = "/v2/"
	LIST_TAGS_ENDPOINT = "/tags/list"
	MANIFEST_ENDPOINT  = "/manifests/"

	headerV1 = "application/vnd.docker.distribution.manifest.v1+prettyjws"
	headerV2 = "application/vnd.docker.distribution.manifest.v2+json"
)

type RegistryClient interface {
	PollingImage(registryURL, repositoryName string, strategy domain.PullingStrategy) (domain.Image, error)
}

type registryClient struct {
	*roundTripper
	strategyFunction map[domain.PullingStrategy]func(tags []string, registryURL string, repositoryName string) (*domain.Image, error)
}

func NewRegistry() RegistryClient {
	registry := &registryClient{
		roundTripper:     newRoundTripper(),
		strategyFunction: map[domain.PullingStrategy]func(tags []string, registryURL string, repositoryName string) (*domain.Image, error){},
	}

	defer registry.setStrategyFunctions()

	return registry
}

func (r *registryClient) setStrategyFunctions() {
	r.strategyFunction[domain.LexicographicStrategy] = r.applyLexicographicStrategy
	r.strategyFunction[domain.LatestByDateStrategy] = r.applyLatestDateStrategy
}

var cachedToken = map[string]struct {
	token     string
	expiresAt time.Time
}{}

func (r *registryClient) getCachedToken(registryURL, repositoryName string) (map[string]struct {
	token     string
	expiresAt time.Time
}, error) {
	token, ok := cachedToken[registryURL+repositoryName]

	if !ok || time.Until(token.expiresAt) < 30*time.Second {

		token, err := r.setToken(registryURL, repositoryName)
		if err != nil {
			return nil, err
		}

		cachedToken[registryURL+repositoryName] = struct {
			token     string
			expiresAt time.Time
		}{
			token:     token.Token,
			expiresAt: time.Now().Add(time.Second * time.Duration(token.ExpiresIn)),
		}
	}

	return cachedToken, nil
}

func (r *registryClient) setToken(registryURL, repositoryName string) (token, error) {
	c := client.New[any]()

	req, err := client.NewRequest(http.MethodGet, "https://"+registryURL+apiVersion, nil)
	if err != nil {
		return token{}, err
	}

	_, httpResponse, err := c.Do(req)
	if err != nil {
		return token{}, err
	}

	if httpResponse != nil {
		if httpResponse.StatusCode == http.StatusUnauthorized {
			header := httpResponse.Header.Get("WWW-Authenticate")

			realm, service := getAuthenticationParams(header)

			req, err := client.NewRequest(http.MethodGet, realm+"?service="+service+"&scope=repository:"+repositoryName+":pull", nil)
			if err != nil {
				return token{}, err
			}

			newClient := client.New[token]()

			response, httpResponse, err := newClient.Do(req)
			if err != nil {
				return token{}, err
			}

			if httpResponse != nil && httpResponse.StatusCode > http.StatusOK {
				var httpError httpError

				if err := json.Unmarshal(httpResponse.Body(), &httpError); err != nil {
					return token{}, err
				}

				return token{}, &httpError
			}

			return *response, nil
		}
	}

	return token{}, nil
}

func (r *registryClient) getTags(registryURL, repositoryName string) (tags, error) {
	token, err := r.getCachedToken(registryURL, repositoryName)
	if err != nil {
		return tags{}, err
	}

	c := client.New[tags]()

	req, err := client.NewRequest(http.MethodGet, "https://"+registryURL+apiVersion+repositoryName+LIST_TAGS_ENDPOINT, nil)
	if err != nil {
		return tags{}, err
	}

	r.addHeader("Authorization", "Bearer "+token[registryURL+repositoryName].token)

	c.Transport = r.RoundTripper

	results, httpResponse, err := c.Do(req)
	if err != nil {
		return tags{}, err
	}

	if httpResponse != nil {
		if httpResponse.StatusCode > http.StatusOK {
			var httpError httpError

			if err := json.Unmarshal(httpResponse.Body(), &httpError); err != nil {
				return tags{}, err
			}

			return tags{}, &httpError
		}
	}

	return *results, nil
}

func (r *registryClient) getLatestManifestForTag(registryURL, repositoryName, tag string) (*domain.Image, error) {
	token, err := r.getCachedToken(registryURL, repositoryName)
	if err != nil {
		return nil, err
	}

	c := client.New[manifestv1]()

	req, err := client.NewRequest(http.MethodGet, "https://"+registryURL+apiVersion+repositoryName+MANIFEST_ENDPOINT+tag, nil)
	if err != nil {
		return nil, err
	}

	r.addHeader("Accept", headerV1)
	r.addHeader("Authorization", "Bearer "+token[registryURL+repositoryName].token)

	c.Transport = r.RoundTripper

	response, httpResponse, err := c.Do(req)
	if err != nil {
		return nil, err
	}

	if httpResponse != nil {
		if httpResponse.StatusCode > http.StatusOK {
			var httpError httpError
			if err := json.Unmarshal(httpResponse.Body(), &httpError); err != nil {
				return nil, err
			}

			return nil, &httpError
		}
	}

	var lastDate time.Time
	for _, history := range response.History {
		var compatibility v1Compatibility
		if err := json.Unmarshal([]byte(history.V1Compatibility), &compatibility); err != nil {
			return nil, err
		}

		manifestDate, err := time.Parse(time.RFC3339Nano, compatibility.Created)
		if err != nil {
			return nil, err
		}

		diff := manifestDate.Sub(lastDate)

		if diff > 0 {
			lastDate = manifestDate
		}
	}

	hash, err := r.getImageHash(registryURL, repositoryName, tag)
	if err != nil {
		return nil, err
	}

	return &domain.Image{
		Tag:       tag,
		Digest:    hash,
		CreatedAt: lastDate,
	}, nil
}

func (r *registryClient) getImageHash(registryURL, repositoryName, tag string) (string, error) {
	token, err := r.getCachedToken(registryURL, repositoryName)
	if err != nil {
		return "", err
	}

	c := client.New[any]()

	req, err := client.NewRequest(http.MethodGet, "https://"+registryURL+apiVersion+repositoryName+MANIFEST_ENDPOINT+tag, nil)
	if err != nil {
		return "", err
	}

	r.addHeader("Authorization", "Bearer "+token[registryURL+repositoryName].token)
	r.addHeader("Accept", headerV2)

	c.Transport = r.RoundTripper

	_, httpResponse, err := c.Do(req)
	if err != nil {
		return "", err
	}

	if httpResponse != nil {
		if httpResponse.StatusCode > http.StatusOK {
			var httpError httpError

			if err := json.Unmarshal(httpResponse.Body(), &httpError); err != nil {
				return "", err
			}

			return "", &httpError
		}
	}

	shaDigest := httpResponse.Header.Get("Docker-Content-Digest")

	return shaDigest, nil
}

func getAuthenticationParams(realmHeader string) (realm string, service string) {
	realm = strings.Split(strings.Split(realmHeader, "realm=\"")[1], "\"")[0]
	service = strings.Split(strings.Split(realmHeader, "service=\"")[1], "\"")[0]
	return realm, service
}

func (r *registryClient) PollingImage(registryURL, repositoryName string, strategy domain.PullingStrategy) (domain.Image, error) {
	tags, err := r.getTags(registryURL, repositoryName)
	if err != nil {
		return domain.Image{}, err
	}

	image, err := r.strategyFunction[strategy](tags.Tags, registryURL, repositoryName)
	if err != nil {
		return domain.Image{}, nil
	}

	return *image, nil
}

func (r *registryClient) applyLatestDateStrategy(tags []string, registryURL, repositoryName string) (*domain.Image, error) {
	results := make(chan *domain.Image, len(tags))

	for _, tag := range tags {
		go func(tag string) {

			manifest, err := r.getLatestManifestForTag(registryURL, repositoryName, tag)
			if err != nil {
				log.Fatal().Err(err).Msg("getting last manifest call failed")
				return
			}
			results <- manifest
		}(tag)
	}

	lastImageCreated := getLatestImageByDate(results, len(tags))

	return &lastImageCreated, nil
}

func (r *registryClient) applyLexicographicStrategy(tags []string, registryURL, repositoryName string) (*domain.Image, error) {
	sort.Strings(tags)

	lastTagReleased := tags[len(tags)-1]

	image, err := r.getLatestManifestForTag(registryURL, repositoryName, lastTagReleased)
	if err != nil {
		return nil, err
	}

	return image, nil
}

func getLatestImageByDate(channel chan *domain.Image, lenTags int) domain.Image {
	var lastImageCreated domain.Image

	for range lenTags {
		manifest := <-channel

		diff := manifest.CreatedAt.Sub(lastImageCreated.CreatedAt)

		if diff > 0 {
			lastImageCreated = *manifest
		}
	}

	return lastImageCreated
}
